package addon

import (
	"github.com/golang/glog"
	"github.com/turbonomic/prometurbo/appmetric/pkg/alligator"
	"github.com/turbonomic/prometurbo/appmetric/pkg/inter"
	xfire "github.com/turbonomic/prometurbo/appmetric/pkg/prometheus"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
)

const (
	// query for transaction per second in the last 10 minutes
	istio_faas_ops_query = `rate(istio_turbo_request_count_by_path[10m])`
	// query for response time in ms in the last 10 minutes
	istio_faas_response_time_query = `1000*rate(istio_turbo_request_duration_by_path_sum[10m])/rate(istio_turbo_request_duration_by_path_count[10m])`
)

// Map of Turbo metric type to Istio for FaaS query
var istioFaaSQueryMap = map[proto.CommodityDTO_CommodityType]string{
	inter.TpsType:     istio_faas_ops_query,
	inter.LatencyType: istio_faas_response_time_query,
}

type IstioFaaSEntityGetter struct {
	name  string
	du    string
	query *istioFaaSQuery
}

// ensure IstioFaaSEntityGetter implement the requisite interfaces
var _ alligator.EntityMetricGetter = &IstioFaaSEntityGetter{}

func NewIstioFaaSEntityGetter(name, du string) *IstioFaaSEntityGetter {
	return &IstioFaaSEntityGetter{
		name: name,
		du:   du,
	}
}

func (r *IstioFaaSEntityGetter) Name() string {
	return r.name
}

func (r *IstioFaaSEntityGetter) Category() string {
	return "Istio for FaaS"
}

func (r *IstioFaaSEntityGetter) GetEntityMetric(client *xfire.RestClient) ([]*inter.EntityMetric, error) {
	result := []*inter.EntityMetric{}
	midResult := make(map[string]*inter.EntityMetric)

	// Get metrics from Prometheus server
	for metricType := range istioFaaSQueryMap {
		query := &istioFaaSQuery{istioFaaSQueryMap[metricType]}
		metrics, err := client.GetMetrics(query)
		if err != nil {
			glog.Errorf("Failed to get Istio metrics for FaaS: %v", err)
			return result, err
		} else {
			err = r.addEntity(metrics, midResult, metricType)
			if err != nil {
				return nil, err
			}
		}
	}

	// Reform map to list
	for _, v := range midResult {
		result = append(result, v)
	}

	return result, nil
}

// addEntity creates entities from the metric data
func (r *IstioFaaSEntityGetter) addEntity(mdat []xfire.MetricData, result map[string]*inter.EntityMetric, key proto.CommodityDTO_CommodityType) error {
	for _, dat := range mdat {
		metric, ok := dat.(*xfire.BasicMetricData)
		if !ok {
			glog.Errorf("Type assertion failed for[%v].", key)
			continue
		}

		srcApp, ok1 := metric.Labels["source_app"]
		srcNs, ok2 := metric.Labels["source_workload_namespace"]
		//dstWorkload, ok3 := metric.Labels["destination_workload"]
		dstNs, ok4 := metric.Labels["destination_workload_namespace"]
		svc, ok5 := metric.Labels["destination_service"]
		svcNs, ok6 := metric.Labels["destination_service_namespace"]
		uri, ok7 := metric.Labels["request_path"]
		respCode, ok8 := metric.Labels["response_code"]

		if !ok1 || !ok2 || !ok4 || !ok5 || !ok6 || !ok7 || !ok8 || respCode != "200" {
			glog.Infof("Some required label not found or response code is not 200 in metric %v", metric)
			continue
		}

		// this prototype only supports using kong as the API gateway
		if dstNs != svcNs || svcNs != "kong" {
			glog.Infof("This prototype supports only kong as the API gateway but this metric is not destined for kong: %v", metric)
			continue
		}

		consumerId := srcNs + "/" + srcApp
		//
		// Destination is some API gateway such as Kong;
		// qualify the provider using src_app + service_host + uri
		// e.g. foo/kong-proxy.kong.svc.cluster.local/hello-kong
		// Read this as: foo calls kong-proxy with path "/hello-kong"
		//
		providerId := srcApp + "/" + svc + uri

		id := consumerId + "-" + providerId
		entity, exist := result[id]
		if !exist {
			entity = inter.NewEntityMetric(id, inter.VAppEntity)
			entity.SetLabel("PRODUCER", providerId)
			entity.SetLabel("CONSUMER", consumerId)
			entity.SetLabel("KEY", svc+uri)
			entity.SetLabel(inter.Category, r.Category())
			result[id] = entity
		}
		entity.SetMetric(key, metric.GetValue())
	}

	return nil
}

//------------------ Get and Parse the metrics ---------------
type istioFaaSQuery struct {
	query string
}

func (q *istioFaaSQuery) GetQuery() string {
	return q.query
}

func (q *istioFaaSQuery) Parse(m *xfire.RawMetric) (xfire.MetricData, error) {
	d := xfire.NewBasicMetricData()
	if err := d.Parse(m); err != nil {
		return nil, err
	}

	return d, nil
}
