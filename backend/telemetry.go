package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// trackServerEvent writes a telemetry event directly to the telemetry_events collection.
// Fire-and-forget — runs in a goroutine with a short timeout.
func trackServerEvent(mongoDB *MongoDB, eventName, userID, tenantID string, props map[string]interface{}) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		event := bson.M{
			"_id":        primitive.NewObjectID(),
			"eventName":  eventName,
			"category":   "custom",
			"properties": props,
			"createdAt":  time.Now(),
		}
		if userID != "" {
			if oid, err := primitive.ObjectIDFromHex(userID); err == nil {
				event["userId"] = oid
			}
		}
		if tenantID != "" {
			if oid, err := primitive.ObjectIDFromHex(tenantID); err == nil {
				event["tenantId"] = oid
			}
		}

		mongoDB.Database.Collection("telemetry_events").InsertOne(ctx, event)
	}()
}
