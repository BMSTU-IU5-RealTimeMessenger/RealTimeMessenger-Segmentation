package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

//TODO: logs

type Server struct {
	HTTPClient  *http.Client
	Destination string
	SegmentSize int
}

func New() (*Server, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	segmentSize, err := strconv.Atoi(os.Getenv("SEGMENT_SIZE"))
	if err != nil {
		return nil, err
	}

	return &Server{
		HTTPClient:  http.DefaultClient,
		Destination: os.Getenv("DESTINATION"),
		SegmentSize: segmentSize,
	}, nil
}

func (s *Server) Run() {
	r := gin.Default()

	r.POST("/segmentation", s.Segmentation)

	r.Run(":8080")
}
func (s *Server) Segmentation(c *gin.Context) {
	var message Message
	if err := c.ShouldBindJSON(&message); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	portions := split(message.Text, s.SegmentSize)

	for _, portion := range portions {
		segment := Segment{
			Data:  string(portion),
			Time:  message.Time,
			Count: len(portions),
		}
		if err := s.send(segment); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	c.Status(http.StatusOK)
}

func main() {
	server, _ := New()
	server.Run()
}

type Message struct {
	Text []byte    `json:"text"`
	Time time.Time `json:"time"`
}

func split(data []byte, chunkSize int) [][]byte {
	var portions [][]byte

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		portions = append(portions, data[i:end])
	}

	return portions
}

type Segment struct {
	Data  string    `json:"data"`
	Time  time.Time `json:"time"`
	Count int       `json:"count"`
}

func (s *Server) send(segment Segment) error {
	jsonData, err := json.Marshal(segment)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", s.Destination, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Unexpected response status: " + resp.Status)
	}
	return nil
}
