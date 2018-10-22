package dtofactory

import (
	"github.com/golang/glog"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/constant"
	"github.com/turbonomic/prometurbo/prometurbo/pkg/discovery/exporter"
	"github.com/turbonomic/turbo-go-sdk/pkg/builder"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
)

type consumerEntityBuilder struct {
	scope string
}

func NewConsumerEntityBuilder(scope string) *consumerEntityBuilder {
	return &consumerEntityBuilder{
		scope:  scope,
	}
}

func (b *consumerEntityBuilder) Build(name string, metrics []*exporter.EntityMetric) (*proto.EntityDTO, error) {
	entityType := metrics[0].Type
	uuid := constant.GetEntityId(entityType, b.scope, name)
	displayName := constant.VAppPrefix + name

	dtoBuilder := builder.NewEntityDTOBuilder(entityType, uuid).
		DisplayName(displayName).
		WithProperty(getEntityProperty(name)).
		Monitored(false)

	for _, metric := range metrics {
		key := metric.Labels["KEY"]
		providerId := constant.GetEntityId(entityType, b.scope, metric.Labels["PRODUCER"])
		provider := builder.CreateProvider(entityType, providerId)
		if transactionUsed, exist := metric.Metrics[proto.CommodityDTO_TRANSACTION]; exist {
			transactionCommodity, err := builder.NewCommodityDTOBuilder(proto.CommodityDTO_TRANSACTION).Key(key).Used(transactionUsed).Create()
			if err != nil {
				glog.Errorf("Error building transaction commodity for entity %s from metric %v: %s", uuid, metric, err)
			} else {
				dtoBuilder.Provider(provider).BuysCommodity(transactionCommodity)
			}
		}
		if responseTimeUsed, exist := metric.Metrics[proto.CommodityDTO_RESPONSE_TIME]; exist {
			responseTimeCommodity, err := builder.NewCommodityDTOBuilder(proto.CommodityDTO_RESPONSE_TIME).Key(key).Used(responseTimeUsed).Create()
			if err != nil {
				glog.Errorf("Error building response time commodity for entity %s from metric %v: %s", uuid, metric, err)
			} else {
				dtoBuilder.Provider(provider).BuysCommodity(responseTimeCommodity)
			}
		}
	}

	entityDto, err := dtoBuilder.ReplacedBy(constant.GetReplacementEntityMetaData()).Create()
	if err != nil {
		glog.Errorf("Error building consumer EntityDTO for entity %s with metrics %v: %s", name, metrics, err)
		return nil, err
	}

	return entityDto, nil
}
