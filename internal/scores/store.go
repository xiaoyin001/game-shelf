// 文件说明：db专属不可变成绩集合，以唯一回合幂等写入并从已确认记录派生榜单和个人最佳。
package scores

import (
	"context"
	"errors"
	"game-shelf/internal/challenge"
	"game-shelf/pkg/contracts"
	"github.com/golang/protobuf/proto"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type document struct {
	ID     string                `bson:"_id"`
	Schema int                   `bson:"schema"`
	Party  string                `bson:"party"`
	Record contracts.ScoreRecord `bson:",inline"`
}
type Store struct{ collection *mongo.Collection }

func NewStore(ctx context.Context, db *mongo.Database) (*Store, error) {
	journal := true
	s := &Store{db.Collection("challenge_results", options.Collection().SetWriteConcern(&writeconcern.WriteConcern{W: "majority", Journal: &journal}).SetReadConcern(readconcern.Majority()))}
	_, err := s.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "rules", Value: 1}, {Key: "mode", Value: 1}, {Key: "day", Value: 1}, {Key: "party", Value: 1}, {Key: "score", Value: -1}, {Key: "end_ms", Value: 1}}},
		{Keys: bson.D{{Key: "members.id", Value: 1}, {Key: "rules", Value: 1}, {Key: "mode", Value: 1}, {Key: "score", Value: -1}, {Key: "end_ms", Value: 1}}},
	})
	return s, err
}
func (s *Store) Save(ctx context.Context, r *contracts.ScoreRecord) error {
	n, party, err := Normalize(r)
	if err != nil {
		return err
	}
	_, err = s.collection.InsertOne(ctx, document{ID: n.RoundID, Schema: 1, Party: party, Record: *n})
	if !mongo.IsDuplicateKeyError(err) {
		return err
	}
	var old document
	if err = s.collection.FindOne(ctx, bson.M{"_id": n.RoundID}).Decode(&old); err != nil {
		return err
	}
	if old.Schema != 1 || old.Party != party || !proto.Equal(&old.Record, n) {
		return ErrConflict
	}
	return nil
}
func (s *Store) Query(ctx context.Context, userID, mode, day string) (*contracts.Leaderboard, error) {
	if !ValidQuery(userID, mode, day) {
		return nil, ErrInvalid
	}
	board := &contracts.Leaderboard{Mode: mode, Day: day, Rules: challenge.Rules, Entries: make([]*contracts.ScoreRecord, 0)}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"schema": 1, "rules": challenge.Rules, "mode": mode, "day": day}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}, {Key: "end_ms", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$party", "entry": bson.M{"$first": "$$ROOT"}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "entry.score", Value: -1}, {Key: "entry.end_ms", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: 20}},
	}
	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row struct {
			Entry document `bson:"entry"`
		}
		if err = cursor.Decode(&row); err != nil {
			return nil, err
		}
		n, party, e := Normalize(&row.Entry.Record)
		if e != nil || party != row.Entry.Party || n.RoundID != row.Entry.ID {
			return nil, ErrInvalid
		}
		board.Entries = append(board.Entries, n)
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	for _, m := range []string{"solo", "duo"} {
		var d document
		err = s.collection.FindOne(ctx, bson.M{"schema": 1, "rules": challenge.Rules, "mode": m, "members.id": userID}, options.FindOne().SetSort(bson.D{{Key: "score", Value: -1}, {Key: "end_ms", Value: 1}, {Key: "_id", Value: 1}})).Decode(&d)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return nil, err
		}
		n, party, e := Normalize(&d.Record)
		if e != nil || party != d.Party || n.RoundID != d.ID {
			return nil, ErrInvalid
		}
		if m == "solo" {
			board.BestSolo = n
		} else {
			board.BestDuo = n
		}
	}
	return board, nil
}
