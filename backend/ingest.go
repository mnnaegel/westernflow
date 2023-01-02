// write a program that takes data from a mongodb collection and then indexes it into elasticsearch
// this program will be run as a cron job
// the program will be run on a server that has access to both mongodb and elasticsearch

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"uwo-tt-api/model"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	// import bson
	"go.mongodb.org/mongo-driver/bson"

	// import elastic
	elastic "github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/viper"
)

func loadEnv() {
	// Load environment configuration
	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()

	if err != nil {
		log.Println("Error in config found!")
		// If loading .env file fails, use variables sourced into the environment
		log.Printf("Error reading config file %s. Using environment variables instead.", err)
	}

	viper.AutomaticEnv()
}

// we will use model.course as the struct for the data we are indexing

func ingest() {
	fmt.Println("Here")
	ctx := context.Background()
	// connect to mongodb
	loadEnv()
	mongoURI,mongoURIOK := viper.Get("MONGO_URI").(string)
	if !mongoURIOK {
		mongoURI = "mongodb://localhost:27018"
	}
	fmt.Println("Attempting to connect to:", mongoURI)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB!")
	defer client.Disconnect(ctx)
	// connect to elasticsearch
	elasticURI, elasticURIOK := viper.Get("ELASTIC_URI").(string)
	// if ELASTIC_URI is not set, use localhost:9200
	if !elasticURIOK {
		elasticURI = "http://localhost:9200"
	}

	fmt.Println("Fetching password...")
	ELASTIC_PASSWORD, ELASTIC_PASSWORDOK := viper.Get("ELASTIC_PASSWORD").(string)
	if !ELASTIC_PASSWORDOK {
		log.Fatal("ELASTIC_PASSWORD not set!")
	}
	fmt.Println("Attempting to connect to:", elasticURI)
	// re-write this line of code with the new library: elasticClient, err := elastic.NewClient(elastic.SetURL(elasticURI))
	cert, _ := ioutil.ReadFile("./http_ca.crt")

	cfg := elastic.Config{
        Addresses: []string{
            "https://localhost:9200",
        },
        Username: "elastic",
        Password: ELASTIC_PASSWORD,
        CACert:   cert,
	}
	elasticClient, err := elastic.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to Elasticsearch!")
	fmt.Println(ELASTIC_PASSWORD)
	// get the collection
	collection := client.Database("uwo-tt-api").Collection("courses")
	fmt.Println("Collection created!")
	// get the cursor
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)
	fmt.Println("Beginning indexing...")
	// loop through the cursor and index the data into elasticsearch
	for cursor.Next(ctx) {
		var course model.Course
		cursor.Decode(&course)
		// fmt.Println("course: ", course)
		// index the data
		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(course)
		if err != nil {
			fmt.Println("Error encoding JSON")
			log.Fatal(err)
		}

		// index the course information into elasticsearch
		res, err := elasticClient.Index(
			"courses",
			&buf,
			elasticClient.Index.WithDocumentID(course.ID),
		)
		if err != nil {
			fmt.Println("Error indexing document")
			log.Fatal(err)
		}
		defer res.Body.Close()
		fmt.Println("A:", res)
	}
}