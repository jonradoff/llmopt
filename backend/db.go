package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri, database string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(10).
		SetMinPoolSize(1).
		SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}

	db.ensureIndexes()
	return db, nil
}

func (m *MongoDB) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
	}

	_, err := m.Analyses().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on analyses: %v", err)
	}

	optIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "analysisId", Value: 1}, {Key: "questionIndex", Value: 1}}},
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
	_, err = m.Optimizations().Indexes().CreateMany(ctx, optIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on optimizations: %v", err)
	}

	todoIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "optimizationId", Value: 1}}},
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "videoAnalysisId", Value: 1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
	_, err = m.Todos().Indexes().CreateMany(ctx, todoIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on todos: %v", err)
	}

	brandIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}}},
	}
	_, err = m.BrandProfiles().Indexes().CreateMany(ctx, brandIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on brand_profiles: %v", err)
	}

	healthIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "checkedAt", Value: -1}}},
	}
	_, err = m.HealthChecks().Indexes().CreateMany(ctx, healthIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on health_checks: %v", err)
	}

	summaryIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}}},
	}
	_, err = m.DomainSummaries().Indexes().CreateMany(ctx, summaryIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on domain_summaries: %v", err)
	}

	videoIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "generatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}}},
	}
	_, err = m.VideoAnalyses().Indexes().CreateMany(ctx, videoIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on video_analyses: %v", err)
	}

	shareIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "shareId", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"shareId": bson.M{"$gt": ""}})},
		{Keys: bson.D{{Key: "visibility", Value: 1}}},
	}
	_, err = m.DomainShares().Indexes().CreateMany(ctx, shareIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on domain_shares: %v", err)
	}

	cacheIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "cacheKey", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0)},
	}
	_, err = m.YouTubeCache().Indexes().CreateMany(ctx, cacheIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on youtube_cache: %v", err)
	}
}

func (m *MongoDB) Analyses() *mongo.Collection {
	return m.Database.Collection("analyses")
}

func (m *MongoDB) Optimizations() *mongo.Collection {
	return m.Database.Collection("optimizations")
}

func (m *MongoDB) Todos() *mongo.Collection {
	return m.Database.Collection("todos")
}

func (m *MongoDB) BrandProfiles() *mongo.Collection {
	return m.Database.Collection("brand_profiles")
}

func (m *MongoDB) HealthChecks() *mongo.Collection {
	return m.Database.Collection("health_checks")
}

func (m *MongoDB) DomainSummaries() *mongo.Collection {
	return m.Database.Collection("domain_summaries")
}

func (m *MongoDB) VideoAnalyses() *mongo.Collection {
	return m.Database.Collection("video_analyses")
}

func (m *MongoDB) YouTubeCache() *mongo.Collection {
	return m.Database.Collection("youtube_cache")
}

func (m *MongoDB) DomainShares() *mongo.Collection {
	return m.Database.Collection("domain_shares")
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

// migrateDomains normalizes domain fields (strips protocol, lowercases) across all collections.
// Safe to run multiple times — only updates documents that have a protocol prefix.
func (m *MongoDB) migrateDomains() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	collections := []string{"analyses", "optimizations", "todos", "brand_profiles", "video_analyses", "domain_summaries"}

	for _, coll := range collections {
		c := m.Database.Collection(coll)
		// Find documents with http:// or https:// in the domain field
		filter := bson.D{{Key: "domain", Value: bson.D{{Key: "$regex", Value: "^https?://"}}}}
		cursor, err := c.Find(ctx, filter)
		if err != nil {
			log.Printf("migrateDomains: error querying %s: %v", coll, err)
			continue
		}
		var updated int
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			oldDomain, ok := doc["domain"].(string)
			if !ok {
				continue
			}
			newDomain := normalizeDomain(oldDomain)
			if newDomain == oldDomain {
				continue
			}
			_, err := c.UpdateByID(ctx, doc["_id"], bson.D{{Key: "$set", Value: bson.D{{Key: "domain", Value: newDomain}}}})
			if err != nil {
				log.Printf("migrateDomains: error updating %s doc %v: %v", coll, doc["_id"], err)
			} else {
				updated++
			}
		}
		cursor.Close(ctx)
		if updated > 0 {
			log.Printf("migrateDomains: normalized %d domains in %s", updated, coll)
		}
	}
}
