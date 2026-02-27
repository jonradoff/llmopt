package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// migrateTenantIDs assigns the root tenant's ID (as a hex string) to all
// existing documents that don't have a tenantId yet, and fixes any documents
// where tenantId was previously stored as an ObjectId (should be a string).
// Safe to run multiple times.
func (m *MongoDB) migrateTenantIDs() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Find the root tenant
	var rootTenant struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err := m.Database.Collection("tenants").FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Println("migrateTenantIDs: no root tenant found — skipping (will run on next startup after bootstrap)")
			return
		}
		log.Printf("migrateTenantIDs: error finding root tenant: %v", err)
		return
	}

	// Store as hex string — matches what the middleware puts in context
	rootIDStr := rootTenant.ID.Hex()
	log.Printf("migrateTenantIDs: root tenant ID = %s", rootIDStr)

	// Collections that are tenant-scoped
	collections := []string{
		"analyses",
		"optimizations",
		"todos",
		"brand_profiles",
		"domain_summaries",
		"video_analyses",
	}

	for _, coll := range collections {
		c := m.Database.Collection(coll)

		// Phase 1: Assign tenantId where missing
		filter := bson.M{"$or": bson.A{
			bson.M{"tenantId": bson.M{"$exists": false}},
			bson.M{"tenantId": ""},
		}}
		update := bson.M{"$set": bson.M{"tenantId": rootIDStr}}
		result, err := c.UpdateMany(ctx, filter, update)
		if err != nil {
			log.Printf("migrateTenantIDs: error updating %s: %v", coll, err)
			continue
		}
		if result.ModifiedCount > 0 {
			log.Printf("migrateTenantIDs: assigned root tenant to %d documents in %s", result.ModifiedCount, coll)
		}

		// Phase 2: Fix documents where tenantId is an ObjectId (should be string).
		// MongoDB $type 7 = ObjectId.
		fixFilter := bson.M{"tenantId": bson.M{"$type": 7}}
		cursor, err := c.Find(ctx, fixFilter)
		if err != nil {
			log.Printf("migrateTenantIDs: error scanning %s for ObjectId tenantIds: %v", coll, err)
			continue
		}
		var fixed int64
		for cursor.Next(ctx) {
			var doc struct {
				ID       primitive.ObjectID `bson:"_id"`
				TenantID primitive.ObjectID `bson:"tenantId"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			tidStr := doc.TenantID.Hex()
			_, err := c.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{"$set": bson.M{"tenantId": tidStr}})
			if err != nil {
				log.Printf("migrateTenantIDs: error fixing ObjectId in %s/%s: %v", coll, doc.ID.Hex(), err)
				continue
			}
			fixed++
		}
		cursor.Close(ctx)
		if fixed > 0 {
			log.Printf("migrateTenantIDs: converted %d ObjectId tenantIds to strings in %s", fixed, coll)
		}
	}

	log.Println("migrateTenantIDs: migration complete")
}

// migratePublicToDomainShares creates DomainShare records for any domains that
// had public: true on optimizations or brand_profiles. Idempotent — skips
// domains that already have a DomainShare record.
func (m *MongoDB) migratePublicToDomainShares() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	type tenantDomain struct {
		TenantID string
		Domain   string
	}
	seen := map[tenantDomain]bool{}

	// Scan optimizations with public: true
	cursor, err := m.Database.Collection("optimizations").Find(ctx, bson.M{"public": true})
	if err != nil {
		log.Printf("migratePublicToDomainShares: error scanning optimizations: %v", err)
		return
	}
	for cursor.Next(ctx) {
		var doc struct {
			TenantID string `bson:"tenantId"`
			Domain   string `bson:"domain"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.TenantID != "" && doc.Domain != "" {
			seen[tenantDomain{doc.TenantID, doc.Domain}] = true
		}
	}
	cursor.Close(ctx)

	// Scan brand_profiles with public: true
	cursor, err = m.Database.Collection("brand_profiles").Find(ctx, bson.M{"public": true})
	if err != nil {
		log.Printf("migratePublicToDomainShares: error scanning brand_profiles: %v", err)
		return
	}
	for cursor.Next(ctx) {
		var doc struct {
			TenantID string `bson:"tenantId"`
			Domain   string `bson:"domain"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.TenantID != "" && doc.Domain != "" {
			seen[tenantDomain{doc.TenantID, doc.Domain}] = true
		}
	}
	cursor.Close(ctx)

	if len(seen) == 0 {
		log.Println("migratePublicToDomainShares: no public domains to migrate")
		return
	}

	var created int
	now := time.Now()
	for td := range seen {
		// Skip if a DomainShare already exists
		count, _ := m.DomainShares().CountDocuments(ctx, bson.M{"tenantId": td.TenantID, "domain": td.Domain})
		if count > 0 {
			continue
		}

		_, err := m.DomainShares().UpdateOne(ctx,
			bson.M{"tenantId": td.TenantID, "domain": td.Domain},
			bson.M{
				"$set": bson.M{
					"visibility": "public",
					"shareId":    generateShareID(),
					"updatedAt":  now,
				},
				"$setOnInsert": bson.M{
					"tenantId":  td.TenantID,
					"domain":    td.Domain,
					"createdAt": now,
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Printf("migratePublicToDomainShares: error creating share for %s/%s: %v", td.TenantID, td.Domain, err)
			continue
		}
		created++
	}

	log.Printf("migratePublicToDomainShares: created %d domain shares from %d public domains", created, len(seen))
}
