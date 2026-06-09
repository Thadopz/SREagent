package client

import (
	"SREagent/utility/common"
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func NewMilvusClient(ctx context.Context) (cli.Client, error) {
	address := milvusAddress(ctx)
	dbName := common.MilvusDB(ctx)
	collectionName := common.MilvusCollection(ctx)
	defaultClient, err := cli.NewClient(ctx, cli.Config{
		Address: address,
		DBName:  "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to default database: %w", err)
	}

	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		defaultClient.Close()
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	agentDBExists := false
	for _, db := range databases {
		if db.Name == dbName {
			agentDBExists = true
			break
		}
	}
	if !agentDBExists {
		err = defaultClient.CreateDatabase(ctx, dbName)
		if err != nil {
			defaultClient.Close()
			return nil, fmt.Errorf("failed to create agent database: %w", err)
		}
	}
	defaultClient.Close()

	agentClient, err := cli.NewClient(ctx, cli.Config{
		Address: address,
		DBName:  dbName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent database: %w", err)
	}

	collections, err := agentClient.ListCollections(ctx)
	if err != nil {
		agentClient.Close()
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	bizCollectionExists := false
	for _, collection := range collections {
		if collection.Name == collectionName {
			bizCollectionExists = true
			break
		}
	}

	if !bizCollectionExists {
		schema := &entity.Schema{
			CollectionName: collectionName,
			Description:    "Business knowledge collection",
			Fields:         fields(ctx),
		}

		err = agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		idIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, collectionName, "id", idIndex, false)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}

		contentIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, collectionName, "content", contentIndex, false)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}

		vectorIndex, err := entity.NewIndexAUTOINDEX(entity.HAMMING)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, collectionName, "vector", vectorIndex, false)
		if err != nil {
			agentClient.Close()
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
	}

	if err = agentClient.LoadCollection(ctx, collectionName, false); err != nil {
		agentClient.Close()
		return nil, fmt.Errorf("failed to load collection: %w", err)
	}

	return agentClient, nil
}

func milvusAddress(ctx context.Context) string {
	v, err := g.Cfg().Get(ctx, "milvus.address")
	if err != nil {
		return "localhost:19530"
	}
	address := strings.TrimSpace(v.String())
	if address == "" {
		return "localhost:19530"
	}
	return address
}

func fields(ctx context.Context) []*entity.Field {
	return []*entity.Field{
		{
			Name:     "id",
			DataType: entity.FieldTypeVarChar,
			TypeParams: map[string]string{
				"max_length": "256",
			},
			PrimaryKey: true,
		},
		{
			Name:     "vector",
			DataType: entity.FieldTypeBinaryVector,
			TypeParams: map[string]string{
				"dim": common.MilvusBinaryVectorDim(ctx),
			},
		},
		{
			Name:     "content",
			DataType: entity.FieldTypeVarChar,
			TypeParams: map[string]string{
				"max_length": "8192",
			},
		},
		{
			Name:     "metadata",
			DataType: entity.FieldTypeJSON,
		},
	}
}
