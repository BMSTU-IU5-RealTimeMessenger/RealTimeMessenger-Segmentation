package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Segment struct {
	Data   string `json:"data"`
	Time   int64  `json:"time"`
	Number int    `json:"number"`
	Count  int    `json:"count"`
}

type Server struct {
	HTTPClient  *http.Client
	Destination string
	SegmentSize int
}

func New() (*Server, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file")
	}

	segmentSize, err := strconv.Atoi(os.Getenv("SEGMENT_SIZE"))
	if err != nil {
		return nil, err
	}

	return &Server{
		HTTPClient:  http.DefaultClient,
		Destination: os.Getenv("CHANNEL_LAYER_ADDR"),
		SegmentSize: segmentSize,
	}, nil
}

func (s *Server) Run() {
	r := gin.Default()

	r.POST("/send", s.Segmentation)

	log.Println("Server is running")
	r.Run(os.Getenv("IP") + ":" + os.Getenv("SEGMENTATION_SERVER_PORT"))
}

func (s *Server) Segmentation(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	log.Println("> ", string(data))
	portions := split(data, s.SegmentSize)

	currentTime := time.Now().UnixMilli()
	for i, portion := range portions {
		segment := Segment{
			Data:   string(portion),
			Time:   currentTime,
			Number: i,
			Count:  len(portions),
		}
		log.Println("<", segment)
		if err := s.send(segment); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	c.Status(http.StatusOK)
}

func main() {
	server, err := New()
	if err != nil {
		log.Fatalln(err)
		return
	}

	server.Run()
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

func (s *Server) send(segment Segment) error {
	jsonData, err := json.Marshal(segment)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "http://"+s.Destination, bytes.NewBuffer(jsonData))
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
