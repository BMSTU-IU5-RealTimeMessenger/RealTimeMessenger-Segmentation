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

func TestSegmentation(t *testing.T) {
	type args struct {
		Data        []byte
		SegmentSize int
		Time        time.Time
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
				Data:        []byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'},
				SegmentSize: 4,
				Time:        time.Date(2024, time.February, 28, 1, 1, 1, 1, time.UTC),
				Segments: []string{
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"count\":%d}", string([]byte{'A', 'B', 'C', 'D'}), "2024-02-28T01:01:01.000000001Z", 3),
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"count\":%d}", string([]byte{'E', 'F', 'G', 'H'}), "2024-02-28T01:01:01.000000001Z", 3),
					fmt.Sprintf("{\"data\":\"%s\",\"time\":\"%s\",\"count\":%d}", string([]byte{'I', 'J'}), "2024-02-28T01:01:01.000000001Z", 3),
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid JSON Input",
			args: args{
				Data:        []byte("Invalid JSON"),
				SegmentSize: 4,
				Time:        time.Date(2024, time.February, 28, 1, 1, 1, 1, time.UTC),
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

			jsonData, err := json.Marshal(Message{
				Text: tt.args.Data,
				Time: tt.args.Time,
			})
			if err != nil {
				t.Fatal(err)
				return
			}

			req, err := http.NewRequest("POST", "/test", bytes.NewBuffer(jsonData))
			if err != nil {
				t.Fatal(err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
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
