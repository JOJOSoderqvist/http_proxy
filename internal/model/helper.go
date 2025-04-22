package model

import (
	"errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func StringToObjectID(id string) (primitive.ObjectID, error) {
	if id == "" {
		return primitive.NilObjectID, errors.New("empty ID string")
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, err
	}

	return objectID, nil
}
