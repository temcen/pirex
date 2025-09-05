package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/temcen/pirex/internal/services"
)

// RealtimeLearningDemo demonstrates the real-time learning system
func main() {
	fmt.Println("🚀 Real-Time Learning System Demo")
	fmt.Println("==================================")

	// This is a demonstration of how the real-time learning system would be used
	// In a real application, you would have actual database and Redis connections

	// Mock dependencies for demo
	var db *sql.DB // Would be actual database connection
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})
	kafkaWriter := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "feedback-events",
		Balancer: &kafka.LeastBytes{},
	}

	// Create the real-time learning service
	learningService := services.NewRealtimeLearningService(db, redisClient, kafkaWriter)

	fmt.Println("✅ Real-time learning service created")

	// Demonstrate feedback processing
	demonstrateFeedbackProcessing(learningService)

	// Demonstrate algorithm optimization
	demonstrateAlgorithmOptimization(learningService)

	// Demonstrate A/B testing
	demonstrateABTesting(learningService)

	// Demonstrate continuous learning
	demonstrateContinuousLearning(learningService)

	// Show system metrics
	demonstrateSystemMetrics(learningService)

	fmt.Println("\n🎉 Demo completed successfully!")
}

func demonstrateFeedbackProcessing(service *services.RealtimeLearningService) {
	fmt.Println("\n📊 Feedback Processing Demo")
	fmt.Println("----------------------------")

	// Simulate various types of user feedback
	feedbackEvents := []services.FeedbackEvent{
		{
			UserID:    "user_001",
			ItemID:    "item_123",
			Type:      services.FeedbackExplicit,
			Action:    "rating",
			Value:     4.5,
			Timestamp: time.Now(),
			SessionID: "session_abc",
			Context: map[string]interface{}{
				"algorithm": "semantic_search",
				"position":  1,
			},
		},
		{
			UserID:    "user_001",
			ItemID:    "item_456",
			Type:      services.FeedbackImplicit,
			Action:    "click",
			Value:     1.0,
			Timestamp: time.Now(),
			SessionID: "session_abc",
			Context: map[string]interface{}{
				"algorithm": "collaborative_filtering",
				"position":  2,
			},
		},
		{
			UserID:    "user_002",
			ItemID:    "item_789",
			Type:      services.FeedbackExplicit,
			Action:    "like",
			Value:     1.0,
			Timestamp: time.Now(),
			SessionID: "session_def",
			Context: map[string]interface{}{
				"algorithm": "pagerank",
				"position":  1,
			},
		},
	}

	for i, event := range feedbackEvents {
		fmt.Printf("Processing feedback event %d: %s %s on %s\n",
			i+1, event.Action, event.Type, event.ItemID)

		// In a real system, this would process the feedback
		// err := service.ProcessFeedback(event)
		// if err != nil {
		//     log.Printf("Error processing feedback: %v", err)
		// }
	}

	fmt.Println("✅ Feedback processing demo completed")
}

func demonstrateAlgorithmOptimization(service *services.RealtimeLearningService) {
	fmt.Println("\n🎯 Algorithm Optimization Demo")
	fmt.Println("-------------------------------")

	userID := "user_001"

	// Get current algorithm weights
	weights := service.GetAlgorithmWeights(userID)
	fmt.Printf("Current algorithm weights for %s:\n", userID)
	for algorithm, weight := range weights {
		fmt.Printf("  %s: %.3f\n", algorithm, weight)
	}

	// Simulate recording algorithm performance
	algorithms := []string{"semantic_search", "collaborative_filtering", "pagerank"}

	fmt.Println("\nRecording algorithm performance:")
	for _, algorithm := range algorithms {
		// Simulate different performance metrics
		impressions := int64(100)
		clicks := int64(15)
		conversions := int64(3)
		satisfaction := 0.8

		fmt.Printf("  %s: %d impressions, %d clicks, %d conversions, %.1f satisfaction\n",
			algorithm, impressions, clicks, conversions, satisfaction)

		// In a real system:
		// err := service.RecordAlgorithmPerformance(algorithm, userID, impressions, clicks, conversions, satisfaction)
		// if err != nil {
		//     log.Printf("Error recording performance: %v", err)
		// }
	}

	fmt.Println("✅ Algorithm optimization demo completed")
}

func demonstrateABTesting(service *services.RealtimeLearningService) {
	fmt.Println("\n🧪 A/B Testing Demo")
	fmt.Println("-------------------")

	// Create a sample experiment
	experiment := &services.Experiment{
		Name:        "Algorithm Comparison Test",
		Description: "Compare semantic search vs collaborative filtering",
		Type:        services.ExperimentTypeAlgorithm,
		Variants: []services.ExperimentVariant{
			{
				ID:                "control",
				Name:              "Semantic Search",
				TrafficAllocation: 0.5,
				IsControl:         true,
				Configuration: map[string]interface{}{
					"algorithm": "semantic_search",
				},
			},
			{
				ID:                "variant_a",
				Name:              "Collaborative Filtering",
				TrafficAllocation: 0.5,
				IsControl:         false,
				Configuration: map[string]interface{}{
					"algorithm": "collaborative_filtering",
				},
			},
		},
		SuccessMetrics:    []string{"ctr", "conversion_rate"},
		MinSampleSize:     1000,
		TargetPower:       0.8,
		SignificanceLevel: 0.05,
	}

	fmt.Printf("Created experiment: %s\n", experiment.Name)
	fmt.Printf("Variants: %d\n", len(experiment.Variants))
	for _, variant := range experiment.Variants {
		fmt.Printf("  %s: %.1f%% traffic\n", variant.Name, variant.TrafficAllocation*100)
	}

	// Simulate user assignments
	users := []string{"user_001", "user_002", "user_003", "user_004", "user_005"}
	fmt.Println("\nUser assignments:")
	for _, userID := range users {
		// In a real system:
		// variantID, err := service.AssignUserToExperiment(userID, experiment.ID)
		// if err != nil {
		//     log.Printf("Error assigning user: %v", err)
		//     continue
		// }
		// fmt.Printf("  %s -> %s\n", userID, variantID)

		// For demo, simulate assignment
		variant := "control"
		if userID > "user_003" {
			variant = "variant_a"
		}
		fmt.Printf("  %s -> %s\n", userID, variant)
	}

	fmt.Println("✅ A/B testing demo completed")
}

func demonstrateContinuousLearning(service *services.RealtimeLearningService) {
	fmt.Println("\n🤖 Continuous Learning Demo")
	fmt.Println("----------------------------")

	userTypes := []struct {
		userID      string
		description string
	}{
		{"new_user_001", "New User (< 5 interactions)"},
		{"power_user_001", "Power User (> 100 interactions)"},
		{"inactive_user_001", "Inactive User (30+ days since last interaction)"},
		{"regular_user_001", "Regular User"},
	}

	fmt.Println("Recommendation strategies by user type:")
	for _, user := range userTypes {
		strategy := service.GetRecommendationStrategy(user.userID)
		shouldExplore := service.ShouldExplore(user.userID)

		fmt.Printf("\n%s (%s):\n", user.description, user.userID)
		fmt.Printf("  Should explore: %v\n", shouldExplore)

		// In a real system, strategy would contain actual values
		fmt.Printf("  Primary algorithm: %v\n", strategy["primary_algorithm"])
		fmt.Printf("  Diversity weight: %v\n", strategy["diversity_weight"])
	}

	fmt.Println("\n✅ Continuous learning demo completed")
}

func demonstrateSystemMetrics(service *services.RealtimeLearningService) {
	fmt.Println("\n📈 System Metrics Demo")
	fmt.Println("----------------------")

	// Get system health
	health := service.HealthCheck()
	fmt.Printf("System health: %s\n", health["overall"])

	// Get system metrics
	metrics := service.GetSystemMetrics()
	fmt.Printf("System status: %v\n", metrics["status"])

	// Show component health
	fmt.Println("\nComponent health:")
	for component, status := range health {
		if component != "overall" {
			fmt.Printf("  %s: %s\n", component, status)
		}
	}

	fmt.Println("✅ System metrics demo completed")
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
