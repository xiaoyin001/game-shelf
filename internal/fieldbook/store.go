// 文件说明：以单文档版本CAS同步保存收藏，确认落盘后才允许世界宣布成功。
package fieldbook

import (
	"context"
	"encoding/hex"
	"errors"
	"game-shelf/pkg/contracts"
	content "game-shelf/pkg/fieldbook"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

var ErrConflict = errors.New("field_book_conflict")

type document struct {
	ID      string            `bson:"_id"`
	Schema  int               `bson:"schema"`
	Journal contracts.Journal `bson:",inline"`
}
type Store struct{ collection *mongo.Collection }

func NewStore(db *mongo.Database) *Store {
	journal := true
	return &Store{collection: db.Collection("field_books", options.Collection().SetWriteConcern(&writeconcern.WriteConcern{W: "majority", Journal: &journal}))}
}
func validID(id string) bool {
	data, err := hex.DecodeString(id)
	return err == nil && len(data) == 16
}

// Load 不为缺失档案写默认数据；不支持或损坏的档案必须显式失败。
func (s *Store) Load(ctx context.Context, userID string) (*contracts.Journal, error) {
	if !validID(userID) {
		return nil, errors.New("invalid_user_id")
	}
	var doc document
	err := s.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &contracts.Journal{}, nil
	}
	if err != nil {
		return nil, err
	}
	if doc.Schema != 1 && doc.Schema != 2 {
		return nil, errors.New("unsupported_field_book_schema")
	}
	if err := content.Validate(&doc.Journal); err != nil {
		return nil, err
	}
	return content.Clone(&doc.Journal), nil
}

// Save 的Revision为预期旧版本，不进行无条件覆盖，超时可以重读确定最终状态。
func (s *Store) Save(ctx context.Context, userID string, next *contracts.Journal) (*contracts.Journal, error) {
	if !validID(userID) {
		return nil, errors.New("invalid_user_id")
	}
	if err := content.Validate(next); err != nil {
		return nil, err
	}
	if next.Revision >= content.MaxRevision {
		return nil, errors.New("field_book_revision_exhausted")
	}
	saved := content.Clone(next)
	saved.Revision++
	result, err := s.collection.ReplaceOne(ctx, bson.M{"_id": userID, "schema": bson.M{"$in": []int{1, 2}}, "revision": next.Revision}, document{ID: userID, Schema: 2, Journal: *saved}, options.Replace().SetUpsert(next.Revision == 0))
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return nil, ErrConflict
	}
	return saved, nil
}
