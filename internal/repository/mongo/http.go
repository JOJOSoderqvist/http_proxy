package mongo

import (
	"context"
	"log"
	"simple_proxy/internal/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HTTPRepository struct {
	client        *mongo.Client
	database      string
	requestsColl  *mongo.Collection
	responsesColl *mongo.Collection
}

func NewHTTPRepository(uri, database string) (*HTTPRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	repo := &HTTPRepository{
		client:        client,
		database:      database,
		requestsColl:  client.Database(database).Collection("requests"),
		responsesColl: client.Database(database).Collection("responses"),
	}

	repo.createIndexes(ctx)

	return repo, nil
}

func (r *HTTPRepository) createIndexes(ctx context.Context) {
	_, err := r.requestsColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "timestamp", Value: -1}},
	})
	if err != nil {
		log.Printf("Error creating timestamp index on requests: %v", err)
	}

	_, err = r.responsesColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "request_id", Value: 1}},
	})
	if err != nil {
		log.Printf("Error creating request_id index on responses: %v", err)
	}
}

func (r *HTTPRepository) SaveRequest(ctx context.Context, request *model.HTTPRequest) error {
	request.ID = primitive.NewObjectID()
	request.Timestamp = time.Now()

	_, err := r.requestsColl.InsertOne(ctx, request)
	return err
}

func (r *HTTPRepository) SaveResponse(ctx context.Context, response *model.HTTPResponse) error {
	response.ID = primitive.NewObjectID()
	response.Timestamp = time.Now()

	_, err := r.responsesColl.InsertOne(ctx, response)
	if err != nil {
		return err
	}

	_, err = r.requestsColl.UpdateOne(
		ctx,
		bson.M{"_id": response.RequestID},
		bson.M{"$set": bson.M{"response_id": response.ID}},
	)
	return err
}

func (r *HTTPRepository) GetTransaction(ctx context.Context, requestID primitive.ObjectID) (*model.HTTPTransaction, error) {
	var request model.HTTPRequest
	err := r.requestsColl.FindOne(ctx, bson.M{"_id": requestID}).Decode(&request)
	if err != nil {
		return nil, err
	}

	var response model.HTTPResponse
	err = r.responsesColl.FindOne(ctx, bson.M{"request_id": requestID}).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &model.HTTPTransaction{
		Request:  request,
		Response: response,
	}, nil
}

func (r *HTTPRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
