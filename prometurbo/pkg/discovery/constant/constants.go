package constant

import (
	"fmt"
	"github.com/turbonomic/turbo-go-sdk/pkg/builder"
	"github.com/turbonomic/turbo-go-sdk/pkg/proto"
)

const (
	// EntityType
	ApplicationType = int32(1)

	// CommodityType
	TPS     = "tps"
	Latency = "latency"

	// MetricType
	Used     = "used"
	Capacity = "capacity"

	// Capacity
	TPSCap     = 20.0
	LatencyCap = 500.0 //millisec

	// The default namespace of entity property
	DefaultPropertyNamespace = "DEFAULT"

	// The attribute used for stitching with other probes with vapp
	StitchingLocalAttr    string = "VappIds"
	StitchingExternalAttr string = "VappIds"

	VAppPrefix = "vApp-"
)

var EntityTypeMap = map[proto.EntityDTO_EntityType]struct{}{
	proto.EntityDTO_APPLICATION: {},
	proto.EntityDTO_VIRTUAL_APPLICATION: {},
}

var CommodityTypeMap = map[proto.CommodityDTO_CommodityType]struct{}{
	proto.CommodityDTO_TRANSACTION:   {},
	proto.CommodityDTO_RESPONSE_TIME: {},
}

var CommodityCapMap = map[proto.CommodityDTO_CommodityType]float64{
	proto.CommodityDTO_TRANSACTION:   TPSCap,
	proto.CommodityDTO_RESPONSE_TIME: LatencyCap,
}

func GetEntityId(entityType proto.EntityDTO_EntityType, scope, entityName string) string {
	return fmt.Sprintf("%s-%s/%s", entityType, scope, entityName)
}

func GetExternalEntityProfDef() *proto.ServerEntityPropDef {
	entityType := proto.EntityDTO_VIRTUAL_APPLICATION
	attr := StitchingExternalAttr
	useTopoExt := true

	return &proto.ServerEntityPropDef{
		Entity:     &entityType,
		Attribute:  &attr,
		UseTopoExt: &useTopoExt,
	}
}

func GetReplacementEntityMetaData() *proto.EntityDTO_ReplacementEntityMetaData {
	return builder.NewReplacementEntityMetaDataBuilder().
		Matching(StitchingLocalAttr).
		MatchingExternal(GetExternalEntityProfDef()).
		PatchBuyingWithProperty(proto.CommodityDTO_TRANSACTION, []string{Used}).
		PatchBuyingWithProperty(proto.CommodityDTO_RESPONSE_TIME, []string{Used}).
		PatchSellingWithProperty(proto.CommodityDTO_TRANSACTION, []string{Used, Capacity}).
		PatchSellingWithProperty(proto.CommodityDTO_RESPONSE_TIME, []string{Used, Capacity}).
		Build()
}