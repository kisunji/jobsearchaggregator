package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kisunji/jobsearchaggregator/jobservice"
)

var (
	// ErrJobService is thrown when there is an issue unmarshalling the json produced by jobservice
	ErrJobService = errors.New("There was an issue with the jobservice API")
)

func main() {
	mode := os.Getenv("MODE")
	switch mode {
	case "lambda":
		log.Print("Running lambda handler")
		lambda.Start(Handler)
	default:
		port, ok := os.LookupEnv("PORT")
		if !ok {
			port = ":80"
		} else {
			log.Printf("Custom port (%s) detected", port)
			port = ":" + port
		}
		http.HandleFunc("/JobSearch", LocalHandler)
		log.Printf("Running locally: localhost%s/JobSearch", port)
		http.ListenAndServe(port, nil)
	}
}

//Handler is the AWS Lambda function handler that uses Amazon API Gateway request/response
func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Processing Lambda request %s\n", request.RequestContext.RequestID)
	bytes, err := getJobs()
	if err != nil {
		return events.APIGatewayProxyResponse{}, ErrJobService
	}
	origin, ok := os.LookupEnv("CORS")
	if !ok {
		origin = "*"
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(bytes),
		Headers:    map[string]string{"Access-Control-Allow-Origin": origin},
	}, nil
}

// LocalHandler handles requests for local testing
func LocalHandler(w http.ResponseWriter, req *http.Request) {
	origin, ok := os.LookupEnv("CORS")
	if !ok {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	bytes, err := getJobs()
	if err != nil {
		http.Error(w, "Error occurred", http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func getJobs() ([]byte, error) {
	defer timeTrack(time.Now(), "getJobs")

	services := []jobservice.JobSearch{
		jobservice.NewAmazonSearch(),
		jobservice.NewLeagueSearch(),
		jobservice.NewShopifySearch(),
	}

	c := make(chan []jobservice.Job)
	for _, v := range services {
		go func(v jobservice.JobSearch) { c <- v.Jobs() }(v)
	}
	var jobArray []jobservice.Job
	for i := 0; i < len(services); i++ {
		result := <-c
		jobArray = append(jobArray, result...)
	}
	log.Printf("Jobs found: %d", len(jobArray))
	return json.Marshal(jobArray)
}

// timeTrack measures time to execute
// Credits: https://blog.stathat.com/2012/10/10/time_any_function_in_go.html
func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
