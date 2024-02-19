package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()

    r.POST("/segmentation", segmentation)

    r.Run(":8080")
}

type Message struct {
    Text []byte   `json:"text"`
    Time time.Time `json:"time"`
}

func segmentation(c *gin.Context) {
    var message Message
    if err := c.ShouldBindJSON(&message); err != nil {
        c.AbortWithError(http.StatusBadRequest, err)
        return
    }

    portions := split(message.Text, 120)

    for _, portion := range portions {
        segment := Segment{
            Data:  portion,
            Time:  message.Time,
            Count: len(portions),
        }
        if err := send(segment); err != nil {
            c.AbortWithError(http.StatusInternalServerError, err)
            return
        }
    }

    c.Status(http.StatusOK)
}

type Segment struct {
    Data  []byte    `json:"data"`
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
