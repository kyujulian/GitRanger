package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestUpload(t *testing.T) {
	//upload
}

func TestRedisEnqueueWhenUpload(t *testing.T) {
	//
}

type MyRequest struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

type MyResponse struct {
	// Define according to the expected response
	Success bool `json:"success"`
}

func main() {
	url := "http://your-microservice-endpoint/path"
	payload := MyRequest{
		Field1: "value1",
		Field2: 123,
	}

	response, err := sendPostRequest(url, payload)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}

	respData, err := handleResponse(response)
	if err != nil {
		fmt.Println("Error handling response:", err)
		return
	}

	fmt.Println("Response:", respData)
}
func sendPostRequest(url string, payload MyRequest) (*http.Response, error) {
	// Marshal the payload into JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Create a new request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return response, nil
}
func handleResponse(response *http.Response) (*MyResponse, error) {
	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Unmarshal the JSON data into your response struct
	var respData MyResponse
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return nil, err
	}

	return &respData, nil
}
