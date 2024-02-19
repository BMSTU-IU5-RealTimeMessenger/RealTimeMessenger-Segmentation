package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type Message struct {
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

func main() {
	r := gin.Default()

	r.POST("/split", split)

	r.Run(":8080")
}

func split(c *gin.Context) {
	var message Message
	if err := c.ShouldBind(&message); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	portions := splitData(message.Text, 120)

	for _, portion := range portions {
		segment := Segment{
			Data:  portion,
			Time:  message.Time,
			Count: len(portions),
		}
		send(segment)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data processed and sent"})
}

type Segment struct {
	Data  string    `json:"data"`
	Time  time.Time `json:"time"`
	Count int       `json:"count"`
}

func send(segment Segment) error {
	url := "http://localhost:8000/api/delivery/"

	jsonData, err := json.Marshal(segment)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Unexpected response status: " + resp.Status)
	}
	return nil
}

func splitData(data string, chunkSize int) []string {
	var portions []string

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		portions = append(portions, data[i:end])
	}

	return portions
}
