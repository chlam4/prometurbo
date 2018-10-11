package addon

import (
	"github.com/golang/glog"
	"github.com/turbonomic/prometurbo/appmetric/pkg/alligator"
	"github.com/turbonomic/prometurbo/appmetric/pkg/inter"
	xfire "github.com/turbonomic/prometurbo/appmetric/pkg/prometheus"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
)

const (
	// query for transaction per second (sum of read and write)
	istio_faas_ops_query = `istio_requests_total`
)

// Map of Turbo metric type to Istio for FaaS query
var istioFaaSQueryMap = map[proto.CommodityDTO_CommodityType]string{
	inter.TpsType: istio_faas_ops_query,
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
			r.addEntity(metrics, midResult, metricType)
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
	srcNsLabel := "source_workload_namespace"
	desNsLabel := "destination_workload_namespace"
	srcSvcLabel := "source_app"
	desSvcLabel := "destination_app"

	for _, dat := range mdat {
		metric, ok := dat.(*xfire.BasicMetricData)
		if !ok {
			glog.Errorf("Type assertion failed for[%v].", key)
			continue
		}

		srcNs, ok1 := metric.Labels[srcNsLabel]
		desNs, ok2 := metric.Labels[desNsLabel]
		srcSvc, ok3 := metric.Labels[srcSvcLabel]
		desSvc, ok4 := metric.Labels[desSvcLabel]
		if !ok1 || !ok2 || !ok3 || !ok4 {
			glog.Errorf("Label not found")
			continue
		}

		consumerId := srcNs + "/" + srcSvc
		providerId := desNs + "/" + desSvc

		//2. add entity metrics
		entity, ok := result[providerId]
		if !ok {
			entity = inter.NewEntityMetric(providerId, inter.VAppEntity)
			entity.SetLabel("CONSUMER", consumerId)
			entity.SetLabel(inter.Category, r.Category())
			result[providerId] = entity
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
