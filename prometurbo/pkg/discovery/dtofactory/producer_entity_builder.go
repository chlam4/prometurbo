package dtofactory

import (
	"github.com/golang/glog"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/constant"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/exporter"
	"github.com/turbonomic/turbo-go-sdk/pkg/builder"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
	"math"
	"strings"
)

type producerEntityBuilder struct {
	scope string
}

func NewProducerEntityBuilder(scope string) *producerEntityBuilder {
	return &producerEntityBuilder{
		scope:  scope,
	}
}

func (b *producerEntityBuilder) Build(name string, metrics []*exporter.EntityMetric) (*proto.EntityDTO, error) {
	entityType := metrics[0].Type
	uuid := constant.GetEntityId(entityType, b.scope, name)
	displayName := constant.VAppPrefix + name
	key := metrics[0].Labels["KEY"]

	// Aggregate the total transaction used for this producer
	//
	totalTransactionUsed := 0.0
	totalResponseTimeUsed := 0.0
	for _, metric := range metrics {
		if transactionUsed, exist := metric.Metrics[proto.CommodityDTO_TRANSACTION]; exist {
			totalTransactionUsed += transactionUsed
			if responseTimeUsed, exist := metric.Metrics[proto.CommodityDTO_RESPONSE_TIME]; exist {
				totalResponseTimeUsed += responseTimeUsed * transactionUsed
			}
		}
	}
	if totalTransactionUsed > 0.0 {
		totalResponseTimeUsed /= totalTransactionUsed
	}

	dtoBuilder := builder.NewEntityDTOBuilder(entityType, uuid).
		DisplayName(displayName).
		WithProperty(getEntityProperty(constant.StitchingLocalAttr, name)).
		Monitored(true)
	// TODO: leaving producers monitored until stitched with Istio route probe and the backend function providers
	if !strings.Contains(name, "kong") {
		dtoBuilder.Monitored(false).ReplacedBy(constant.GetReplacementEntityMetaData())
	}

	// Transaction commodity
	//
	transactionCapacity := math.Max(constant.TPSCap, totalTransactionUsed)	// Adjust capacity in case utilization > 1
	// TODO: for demo only - emulating a lower transaction capacity in a cell tower to trigger moves
	if strings.Contains(name, "foo") {
		transactionCapacity = math.Max(5, totalTransactionUsed)
	}
	// as Market doesn't allow it
	transactionCommodity, err := builder.NewCommodityDTOBuilder(proto.CommodityDTO_TRANSACTION).
		Used(totalTransactionUsed).Capacity(transactionCapacity).Key(key).Create()
	if err != nil {
		glog.Errorf("Error building transaction commodity for entity %s with used %v, capacity %v and key %s: %s",
			uuid, totalTransactionUsed, transactionCapacity, name, err)
	} else {
		dtoBuilder.SellsCommodity(transactionCommodity)
	}
	//
	// ResponseTime commodity
	//
	responseTimeCapacity := math.Max(constant.LatencyCap, totalResponseTimeUsed)
	responseTimeCommodity, err := builder.NewCommodityDTOBuilder(proto.CommodityDTO_RESPONSE_TIME).
		Used(totalResponseTimeUsed).Capacity(responseTimeCapacity).Key(key).Create()
	if err != nil {
		glog.Errorf("Error building response time commodity for entity %s with used %v, capacity %v and key %s: %s",
			uuid, totalResponseTimeUsed, responseTimeCapacity, name, err)
	} else {
		dtoBuilder.SellsCommodity(responseTimeCommodity)
	}

	entityDto, err := dtoBuilder.Create()
	if err != nil {
		glog.Errorf("Error building producer EntityDTO for entity %s with metrics %v: %s", name, metrics, err)
		return nil, err
	}

	return entityDto, nil
}
