package common

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	MilvusDBName         = "agent"
	MilvusCollectionName = "biz"
	MilvusVectorDim      = 65536
)

var FileDir = "./docs/"

func MilvusDB(ctx context.Context) string {
	v, err := g.Cfg().Get(ctx, "milvus.db_name")
	if err != nil {
		return MilvusDBName
	}
	dbName := strings.TrimSpace(v.String())
	if dbName == "" {
		return MilvusDBName
	}
	return dbName
}

func MilvusCollection(ctx context.Context) string {
	v, err := g.Cfg().Get(ctx, "milvus.collection")
	if err != nil {
		return MilvusCollectionName
	}
	collection := strings.TrimSpace(v.String())
	if collection == "" {
		return MilvusCollectionName
	}
	return collection
}

func MilvusBinaryVectorDim(ctx context.Context) string {
	v, err := g.Cfg().Get(ctx, "milvus.vector_dim")
	if err != nil {
		return strconv.Itoa(MilvusVectorDim)
	}
	dim := v.Int()
	if dim <= 0 {
		return strconv.Itoa(MilvusVectorDim)
	}
	return strconv.Itoa(dim)
}
