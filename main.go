package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection

type community_post struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Image       *string            `json:"image,omitempty"`
	Date        time.Time          `json:"date" bson:"date"`
}

func main() {
	fmt.Println("Hello World ")

	err := godotenv.Load(".env")

	if err != nil {
		fmt.Println("Error loading .env file")
	}

	MONGO_URI := os.Getenv("MONGODB_URI")
	clientOptions := options.Client().ApplyURI(MONGO_URI)
	client, err := mongo.Connect(context.Background(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	defer client.Disconnect(context.Background())

	err = client.Ping(context.Background(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB")

	collection = client.Database("info_share").Collection("community_post")

	app := fiber.New()

	app.Post("/api/posts", createPost)
	app.Get("/api/posts", getPosts)
	app.Get("/api/posts/:id", getPost)
	app.Patch("/api/posts/:id", updatePost)
	app.Delete("/api/posts/:id", deletePost)

	port := os.Getenv("PORT")

	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen("0.0.0.0:" + port))
}

func createPost(c *fiber.Ctx) error {
	post := new(community_post)

	if err := c.BodyParser(post); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if post.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required"})
	}

	if post.Description == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Description is required"})
	}

	post.Date = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	insertResult, err := collection.InsertOne(ctx, post)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create post"})
	}

	post.ID = insertResult.InsertedID.(primitive.ObjectID)

	return c.Status(201).JSON(post)
}

func getPosts(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find all posts
	cursor, err := collection.Find(ctx, primitive.D{{}})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch posts", "details": err.Error()})
	}
	defer func() {
		if err = cursor.Close(ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}()

	var posts []community_post

	// Iterate through the cursor and decode each document
	if err = cursor.All(ctx, &posts); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode posts", "details": err.Error()})
	}

	return c.Status(200).JSON(posts)
}

func getPost(c *fiber.Ctx) error {
	// Get the ID from URL parameter
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Post ID is required"})
	}

	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid post ID format"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find the post by ID
	var post community_post
	err = collection.FindOne(ctx, primitive.M{"_id": objectID}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(404).JSON(fiber.Map{"error": "Post not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch post", "details": err.Error()})
	}

	return c.Status(200).JSON(post)
}

func updatePost(c *fiber.Ctx) error {
	// Get the ID from URL parameter
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Post ID is required"})
	}

	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid post ID format"})
	}

	// Parse the request body
	var updates map[string]interface{}
	if err = c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Remove fields that shouldn't be updated
	delete(updates, "_id")
	delete(updates, "date")

	// Check if there are fields to update
	if len(updates) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No valid fields to update"})
	}

	// Add updated timestamp
	updates["date"] = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update the post
	result, err := collection.UpdateOne(
		ctx,
		primitive.M{"_id": objectID},
		primitive.M{"$set": updates},
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update post", "details": err.Error()})
	}

	// Check if any document was matched/modified
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Post not found"})
	}

	// Fetch the updated post
	var updatedPost community_post
	err = collection.FindOne(ctx, primitive.M{"_id": objectID}).Decode(&updatedPost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch updated post", "details": err.Error()})
	}

	return c.Status(200).JSON(updatedPost)
}

func deletePost(c *fiber.Ctx) error {
	// Get the ID from URL parameter
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Post ID is required"})
	}

	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid post ID format"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete the post
	result, err := collection.DeleteOne(ctx, primitive.M{"_id": objectID})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete post", "details": err.Error()})
	}

	// Check if any document was deleted
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Post not found"})
	}

	return c.Status(200).JSON(fiber.Map{"message": "Post deleted successfully"})
}
