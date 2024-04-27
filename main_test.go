package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		chunkSize int
		want      [][]byte
	}{
		{
			name:      "even split",
			data:      []byte("123456"),
			chunkSize: 2,
			want:      [][]byte{[]byte("12"), []byte("34"), []byte("56")},
		},
		{
			name:      "uneven split",
			data:      []byte("12345"),
			chunkSize: 2,
			want:      [][]byte{[]byte("12"), []byte("34"), []byte("5")},
		},
		{
			name:      "single character",
			data:      []byte("1"),
			chunkSize: 2,
			want:      [][]byte{[]byte("1")},
		},
		{
			name:      "empty input",
			data:      []byte(""),
			chunkSize: 2,
			want:      nil,
		},
		{
			name:      "chunk size larger than input",
			data:      []byte("123"),
			chunkSize: 4,
			want:      [][]byte{[]byte("123")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := split(tt.data, tt.chunkSize); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("split() = %v, want %v", got, tt.want)
			}
		})
	}
}

// CreateMessage takes data and a timestamp, marshals them into a Message struct, and returns the JSON representation as a string.
// It returns an error if marshaling fails.
func CreateMessage(data []byte, timestamp time.Time) string {
	message := Message{
		Text: data,
		Time: timestamp,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		panic(fmt.Sprintf("Failed to create request data: %v", err))
	}

	return string(jsonData)
}

func TestSegmentation(t *testing.T) {
	type args struct {
		RequestBody string
		SegmentSize int
		Segments    []string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Ok",
			args: args{
				RequestBody: CreateMessage([]byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'}, time.Date(2024, time.February, 28, 1, 1, 1, 1, time.UTC)),
				SegmentSize: 4,
				Segments: []string{
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"number\":%d,\"count\":%d}", string([]byte{'A', 'B', 'C', 'D'}), "2024-02-28T01:01:01.000000001Z", 0, 3),
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"number\":%d,\"count\":%d}", string([]byte{'E', 'F', 'G', 'H'}), "2024-02-28T01:01:01.000000001Z", 1, 3),
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"number\":%d,\"count\":%d}", string([]byte{'I', 'J'}), "2024-02-28T01:01:01.000000001Z", 2, 3),
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid JSON Input",
			args: args{
				RequestBody: "Invalid JSON",
				SegmentSize: 4,
				Segments:    nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTPClient := MockHTTPClient{}
			server := &Server{HTTPClient: &http.Client{
				Transport: &mockHTTPClient,
			},
				Destination: "test",
				SegmentSize: tt.args.SegmentSize,
			}

			router := gin.New()
			router.POST("/test", server.Segmentation)

			req, err := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte(tt.args.RequestBody)))
			if err != nil {
				t.Fatal(err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				if tt.wantErr {
					return
				}
				t.Errorf("Unexpected status code: got %v, want %v", w.Code, http.StatusOK)
				return
			}

			if len(mockHTTPClient.requests) != len(tt.args.Segments) {
				t.Error("wrong number of segments:", len(mockHTTPClient.requests), "instead of", len(tt.args.Segments))
				return
			}

			for i, req := range mockHTTPClient.requests {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					http.Error(w, "Error reading request body", http.StatusInternalServerError)
					return
				}

				if string(body) != tt.args.Segments[i] {
					t.Error("\nerror in segment,", i, "\nexpected\t", tt.args.Segments[i], "\ngot\t\t", string(body))
					return
				}
			}
		})
	}
}

type MockHTTPClient struct {
	requests []*http.Request
}

func (c *MockHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, req)

	return &http.Response{StatusCode: http.StatusOK}, nil
}
