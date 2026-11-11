package files

import (
	"context"
	"errors"
	"github.com/iwind/TeaGo/actions"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
	"google.golang.org/grpc"
)

type testFileStore struct {
	public       bool
	chunkLookups int
}

func (s *testFileStore) FindEnabledFile(context.Context, *pb.FindEnabledFileRequest, ...grpc.CallOption) (*pb.FindEnabledFileResponse, error) {
	return &pb.FindEnabledFileResponse{File: &pb.File{Id: 42, IsPublic: s.public, Filename: "asset.html", MimeType: "text/html", Size: 6}}, nil
}
func (s *testFileStore) FindAllFileChunkIds(context.Context, *pb.FindAllFileChunkIdsRequest, ...grpc.CallOption) (*pb.FindAllFileChunkIdsResponse, error) {
	s.chunkLookups++
	return &pb.FindAllFileChunkIdsResponse{FileChunkIds: []int64{1}}, nil
}
func (s *testFileStore) DownloadFileChunk(context.Context, *pb.DownloadFileChunkRequest, ...grpc.CallOption) (*pb.DownloadFileChunkResponse, error) {
	return &pb.DownloadFileChunkResponse{FileChunk: &pb.FileChunk{Data: []byte("secret")}}, nil
}

func TestPrivateFilesNeverLeakThroughPublicDownload(t *testing.T) {
	store := &testFileStore{}
	response := httptest.NewRecorder()
	err := downloadFile(context.Background(), response.Header(), response, store, store, 42, false)
	if !errors.Is(err, errFileForbidden) {
		t.Fatalf("private lookup error: %v", err)
	}
	if store.chunkLookups != 0 || response.Body.Len() != 0 {
		t.Fatal("private file chunks accessed before authorization")
	}
}

func TestPublicAndAuthorizedDownloadsAreAttachments(t *testing.T) {
	for _, public := range []bool{true, false} {
		store := &testFileStore{public: public}
		response := httptest.NewRecorder()
		if err := downloadFile(context.Background(), response.Header(), response, store, store, 42, !public); err != nil {
			t.Fatal(err)
		}
		if response.Body.String() != "secret" {
			t.Fatal("authorized contents were not returned")
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Content-Disposition") != "attachment; filename=asset.html" {
			t.Fatal("active file content not isolated as attachment")
		}
	}
}

func TestPrivateDownloadRejectsAnonymousBeforeRPC(t *testing.T) {
	response := httptest.NewRecorder()
	action := &PrivateFileAction{}
	action.Context = actions.NewActionContext()
	action.Request = httptest.NewRequest(http.MethodGet, "http://admin.example/files/private?fileId=42", nil)
	action.ResponseWriter = response
	action.RunGet(struct{ FileId int64 }{42})
	if response.Code != http.StatusForbidden || response.Body.Len() != 0 {
		t.Fatal("anonymous private download did not return hard forbidden response")
	}
}
