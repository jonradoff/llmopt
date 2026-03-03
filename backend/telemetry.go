package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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

// seedEventDefinitions creates or updates event definitions with descriptions and
// parent dependencies for Sankey visualization. Runs once at startup.
func seedEventDefinitions(mongoDB *MongoDB) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	coll := mongoDB.Database.Collection("event_definitions")

	// Define events with descriptions and parent references (by name).
	// parentName "" means root node (no parent).
	type def struct {
		name        string
		description string
		parentName  string
	}

	// Customer journey flow for Sankey:
	//
	// page.view
	//   → analyze.start
	//       → analyze.complete
	//           → optimize.start → optimize.complete
	//           → brand.save
	//           → video.start → video.complete
	//           → reddit.start → reddit.complete
	//           → search.start → search.complete
	//           → test.start → test.complete
	//           → summary.complete
	//               → share.visibility_change → share.link_copied
	//               → todo.status_change
	//
	defs := []def{
		// --- Entry ---
		{"custom.page.view", "User visits the app (any page load, anonymous or authenticated)", ""},

		// --- Core analysis funnel ---
		{"custom.report.analyze.start", "User submits a domain URL to begin site analysis", "custom.page.view"},
		{"custom.report.analyze.complete", "Site analysis finishes and results are displayed", "custom.report.analyze.start"},

		// --- Deepening engagement: reports branch from analysis ---
		{"custom.report.optimize.start", "User initiates an answer optimization report for a question", "custom.report.analyze.complete"},
		{"custom.report.optimize.complete", "Answer optimization report finishes with a score", "custom.report.optimize.start"},

		{"custom.brand.save", "User saves brand intelligence configuration (competitors, queries)", "custom.report.analyze.complete"},

		{"custom.report.video_discover.start", "User starts YouTube video discovery for a domain", "custom.report.analyze.complete"},
		{"custom.report.video_discover.complete", "Video discovery returns matching YouTube videos", "custom.report.video_discover.start"},
		{"custom.report.video.start", "User starts full video authority analysis on selected videos", "custom.report.video_discover.complete"},
		{"custom.report.video.complete", "Video authority analysis finishes with pillar scores", "custom.report.video.start"},

		{"custom.report.reddit.start", "User starts Reddit authority analysis for a domain", "custom.report.analyze.complete"},
		{"custom.report.reddit.complete", "Reddit authority analysis finishes with community insights", "custom.report.reddit.start"},

		{"custom.report.search.start", "User starts search visibility analysis for a domain", "custom.report.analyze.complete"},
		{"custom.report.search.complete", "Search visibility analysis finishes with crawler and AIO scores", "custom.report.search.start"},

		{"custom.report.test.start", "User starts LLM brand test across AI providers", "custom.report.analyze.complete"},
		{"custom.report.test.complete", "LLM brand test finishes with provider comparison results", "custom.report.test.start"},

		// --- Summary and sharing (signals deep engagement) ---
		{"custom.report.summary.start", "User triggers executive summary generation for a domain", "custom.report.analyze.complete"},
		{"custom.report.summary.complete", "Executive summary finishes, consolidating all reports", "custom.report.summary.start"},

		{"custom.share.visibility_change", "User toggles share visibility (public/private) for a domain", "custom.report.summary.complete"},
		{"custom.share.link_copied", "User copies a share link to distribute the report", "custom.share.visibility_change"},

		{"custom.todo.status_change", "User updates a recommended action item status", "custom.report.summary.complete"},

		// --- Subscription gate (conversion signal) ---
		{"custom.funnel.subscription_gate", "User hits the subscription paywall when trying to analyze", "custom.report.analyze.start"},
	}

	// Build a name→ID map from existing definitions.
	nameToID := map[string]primitive.ObjectID{}
	cursor, err := coll.Find(ctx, bson.M{})
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var existing struct {
				ID   primitive.ObjectID `bson:"_id"`
				Name string             `bson:"name"`
			}
			if cursor.Decode(&existing) == nil {
				nameToID[existing.Name] = existing.ID
			}
		}
	}

	now := time.Now()

	// First pass: upsert all definitions (without parents) to ensure IDs exist.
	for _, d := range defs {
		if _, exists := nameToID[d.name]; exists {
			continue
		}
		newID := primitive.NewObjectID()
		doc := bson.M{
			"_id":         newID,
			"name":        d.name,
			"description": d.description,
			"createdAt":   now,
			"updatedAt":   now,
		}
		if _, err := coll.InsertOne(ctx, doc); err == nil {
			nameToID[d.name] = newID
		}
	}

	// Second pass: set descriptions and parent dependencies.
	for _, d := range defs {
		id, ok := nameToID[d.name]
		if !ok {
			continue
		}

		update := bson.M{
			"description": d.description,
			"updatedAt":   now,
		}

		if d.parentName != "" {
			if parentID, ok := nameToID[d.parentName]; ok {
				update["parentId"] = parentID
			}
		}

		coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update}, options.Update().SetUpsert(false))
	}

	log.Printf("Event definitions seeded: %d definitions", len(defs))
}
