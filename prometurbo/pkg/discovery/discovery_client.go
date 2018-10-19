package discovery

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/dtofactory"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/exporter"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/registration"
	"github.com/turbonomic/turbo-go-sdk/pkg/probe"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
)

// Implements the TurboDiscoveryClient interface
type P8sDiscoveryClient struct {
	targetAddr      string
	scope           string
	metricExporters []exporter.MetricExporter
}

func NewDiscoveryClient(targetAddr, scope string, metricExporters []exporter.MetricExporter) *P8sDiscoveryClient {
	return &P8sDiscoveryClient{
		targetAddr:      targetAddr,
		scope:           scope,
		metricExporters: metricExporters,
	}
}

// Get the Account Values to create VMTTarget in the turbo server corresponding to this client
func (d *P8sDiscoveryClient) GetAccountValues() *probe.TurboTargetInfo {
	targetId := registration.TargetIdField
	targetIdVal := &proto.AccountValue{
		Key:         &targetId,
		StringValue: &d.targetAddr,
	}

	scope := registration.Scope
	scopeVal := &proto.AccountValue{
		Key:         &scope,
		StringValue: &d.scope,
	}

	accountValues := []*proto.AccountValue{
		targetIdVal,
		scopeVal,
	}

	targetInfo := probe.NewTurboTargetInfoBuilder(registration.ProbeCategory, registration.TargetType(d.targetAddr),
		registration.TargetIdField, accountValues).Create()

	return targetInfo
}

// Validate the Target
func (d *P8sDiscoveryClient) Validate(accountValues []*proto.AccountValue) (*proto.ValidationResponse, error) {
	validationResponse := &proto.ValidationResponse{}

	// Validation fails if no exporter responses
	for _, metricExporter := range d.metricExporters {
		if metricExporter.Validate() {
			return validationResponse, nil
		}

		glog.Errorf("Unable to connect to metric exporter %v", metricExporter)
	}
	return d.failValidation(), nil
}

// Discover the Target Topology
func (d *P8sDiscoveryClient) Discover(accountValues []*proto.AccountValue) (*proto.DiscoveryResponse, error) {
	glog.V(2).Infof("Discovering the target %s", accountValues)
	var entities []*proto.EntityDTO
	allExportersFailed := true

	for _, metricExporter := range d.metricExporters {
		dtos, err := d.buildEntities(metricExporter)
		if err != nil {
			glog.Errorf("Error while querying metrics exporter %v: %v", metricExporter, err)
			continue
		}
		allExportersFailed = false
		entities = append(entities, dtos...)

		glog.V(4).Infof("Entities built from exporter %v: %v", metricExporter, dtos)
	}

	// The discovery fails if all queries to exporters fail
	if allExportersFailed {
		return d.failDiscovery(), nil
	}

	discoveryResponse := &proto.DiscoveryResponse{
		EntityDTO: entities,
	}

	return discoveryResponse, nil
}

func (d *P8sDiscoveryClient) buildEntities(metricExporter exporter.MetricExporter) ([]*proto.EntityDTO, error) {
	var entities []*proto.EntityDTO

	metrics, err := metricExporter.Query()
	if err != nil {
		glog.Errorf("Error while querying metrics exporter: %v", err)
		return nil, err
	}

	// Generate two maps -
	// 1) Map of producer vs the list of metrics by this producer
	// 2. Map of consumer vs the list of metrics by this consumer
	//
	producerMap := make(map[string][]*exporter.EntityMetric)
	consumerMap := make(map[string][]*exporter.EntityMetric)
	for _, metric := range metrics {
		//
		// producer
		//
		if producerId, ok := metric.Labels["PRODUCER"]; ok {
			metricList, exist := producerMap[producerId]
			if !exist {
				metricList = []*exporter.EntityMetric{}
			}
			metricList = append(metricList, metric)
			producerMap[producerId] = metricList
		}
		//
		// consumer
		//
		if consumerId, ok := metric.Labels["CONSUMER"]; ok {
			metricList, exist := consumerMap[consumerId]
			if !exist {
				metricList = []*exporter.EntityMetric{}
			}
			metricList = append(metricList, metric)
			consumerMap[consumerId] = metricList
		}
	}
	glog.V(4).Infof("Producer map: %v", producerMap)
	glog.V(4).Infof("Consumer map: %v", consumerMap)

	producerBuilder := dtofactory.NewProducerEntityBuilder(d.scope)
	for producerId, metrics := range producerMap {
		dto, err := producerBuilder.Build(producerId, metrics)
		if err != nil {
			glog.Errorf("Error building entity for producer %s with metrics %v: %s", producerId, metrics, err)
			continue
		}
		entities = append(entities, dto)
	}

	consumerBuilder := dtofactory.NewConsumerEntityBuilder(d.scope)
	for consumerId, metrics := range consumerMap {
		dto, err := consumerBuilder.Build(consumerId, metrics)
		if err != nil {
			glog.Errorf("Error building entity for consumer %s with metrics %v: %s", consumerId, metrics, err)
			continue
		}
		entities = append(entities, dto)
	}

	return entities, nil
}

func (d *P8sDiscoveryClient) failDiscovery() *proto.DiscoveryResponse {
	description := fmt.Sprintf("All exporter queries failed: %v", d.metricExporters)
	glog.Errorf(description)
	severity := proto.ErrorDTO_CRITICAL
	errorDTO := &proto.ErrorDTO{
		Severity:    &severity,
		Description: &description,
	}
	discoveryResponse := &proto.DiscoveryResponse{
		ErrorDTO: []*proto.ErrorDTO{errorDTO},
	}
	return discoveryResponse
}

func (d *P8sDiscoveryClient) failValidation() *proto.ValidationResponse {
	description := fmt.Sprintf("All exporter queries failed: %v", d.metricExporters)
	glog.Errorf(description)
	severity := proto.ErrorDTO_CRITICAL
	errorDto := &proto.ErrorDTO{
		Severity:    &severity,
		Description: &description,
	}

	validationResponse := &proto.ValidationResponse{
		ErrorDTO: []*proto.ErrorDTO{errorDto},
	}
	return validationResponse
}
