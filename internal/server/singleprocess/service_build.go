package singleprocess

import (
	"context"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mitchellh/devflow/internal/server"
	pb "github.com/mitchellh/devflow/internal/server/gen"
)

var buildBucket = []byte("build")

func init() {
	dbBuckets = append(dbBuckets, buildBucket)
}

func (s *service) CreateBuild(
	ctx context.Context,
	req *pb.CreateBuildRequest,
) (*pb.CreateBuildResponse, error) {
	id, err := server.Id()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "uuid generation failed: %s", err)
	}

	// Create the build
	build := &pb.Build{
		Id: id,
		Status: &pb.Status{
			State:     pb.Status_RUNNING,
			StartTime: ptypes.TimestampNow(),
		},
		Component: req.Component,
	}

	// Insert into our database
	err = s.db.Update(func(tx *bolt.Tx) error {
		return dbPut(tx.Bucket(buildBucket), build.Id, build)
	})
	if err != nil {
		return nil, err
	}

	return &pb.CreateBuildResponse{Id: id}, nil
}

func (s *service) CompleteBuild(
	ctx context.Context,
	req *pb.CompleteBuildRequest,
) (*empty.Empty, error) {
	return &empty.Empty{}, s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(buildBucket)

		// Read our build
		var build pb.Build
		if err := dbGet(bucket, req.Id, &build); err != nil {
			return err
		}

		// Regardless of the outcome, we update our completion time.
		build.Status.CompleteTime = ptypes.TimestampNow()

		switch result := req.Result.(type) {
		case *pb.CompleteBuildRequest_Artifact:
			build.Status.State = pb.Status_SUCCESS
			build.Artifact = result.Artifact

		case *pb.CompleteBuildRequest_Error:
			build.Status.State = pb.Status_ERROR
			build.Status.Error = result.Error
		}

		// Save
		return dbPut(bucket, build.Id, &build)
	})
}

func (s *service) ListBuilds(
	ctx context.Context,
	req *empty.Empty,
) (*pb.ListBuildsResponse, error) {
	var result []*pb.Build
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(buildBucket)
		return bucket.ForEach(func(k, v []byte) error {
			var build pb.Build
			if err := proto.Unmarshal(v, &build); err != nil {
				panic(err)
			}

			result = append(result, &build)
			return nil
		})
	})

	return &pb.ListBuildsResponse{Builds: result}, nil
}