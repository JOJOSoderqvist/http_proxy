package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HTTPRequest struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Method      string              `bson:"method" json:"method"`
	Path        string              `bson:"path" json:"path"`
	QueryParams map[string][]string `bson:"query_params" json:"query_params"`
	Headers     map[string][]string `bson:"headers" json:"headers"`
	Cookies     map[string]string   `bson:"cookies" json:"cookies"`
	Body        string              `bson:"body" json:"body"`
	FormParams  map[string][]string `bson:"form_params,omitempty" json:"form_params,omitempty"`
	IsGzipped   bool                `bson:"is_gzipped" json:"is_gzipped"`
	TargetHost  string              `bson:"target_host" json:"target_host"`
	ClientIP    string              `bson:"client_ip" json:"client_ip"`
	Timestamp   time.Time           `bson:"timestamp" json:"timestamp"`
	ResponseID  primitive.ObjectID  `bson:"response_id,omitempty" json:"response_id,omitempty"`
}

type HTTPResponse struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	RequestID     primitive.ObjectID  `bson:"request_id" json:"request_id"`
	StatusCode    int                 `bson:"status_code" json:"status_code"`
	Headers       map[string][]string `bson:"headers" json:"headers"`
	Body          string              `bson:"body" json:"body"`
	IsGzipped     bool                `bson:"is_gzipped" json:"is_gzipped"`
	ContentType   string              `bson:"content_type" json:"content_type"`
	ContentLength int64               `bson:"content_length" json:"content_length"`
	Timestamp     time.Time           `bson:"timestamp" json:"timestamp"`
}

type HTTPTransaction struct {
	Request  HTTPRequest  `bson:"request" json:"request"`
	Response HTTPResponse `bson:"response" json:"response"`
}
