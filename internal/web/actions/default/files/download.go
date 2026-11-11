package files

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
	"google.golang.org/grpc"
)

var errFileNotFound = errors.New("file not found")
var errFileForbidden = errors.New("private file requires authenticated access")

type fileLookup interface {
	FindEnabledFile(context.Context, *pb.FindEnabledFileRequest, ...grpc.CallOption) (*pb.FindEnabledFileResponse, error)
}

type chunkLookup interface {
	FindAllFileChunkIds(context.Context, *pb.FindAllFileChunkIdsRequest, ...grpc.CallOption) (*pb.FindAllFileChunkIdsResponse, error)
	DownloadFileChunk(context.Context, *pb.DownloadFileChunkRequest, ...grpc.CallOption) (*pb.DownloadFileChunkResponse, error)
}

// downloadFile applies the public/private boundary before looking up any chunks.
func downloadFile(ctx context.Context, header http.Header, writer io.Writer, files fileLookup, chunks chunkLookup, fileID int64, allowPrivate bool) error {
	response, err := files.FindEnabledFile(ctx, &pb.FindEnabledFileRequest{FileId: fileID})
	if err != nil {
		return err
	}
	file := response.File
	if file == nil {
		return errFileNotFound
	}
	if !file.IsPublic && !allowPrivate {
		return errFileForbidden
	}
	ids, err := chunks.FindAllFileChunkIds(ctx, &pb.FindAllFileChunkIdsRequest{FileId: file.Id})
	if err != nil {
		return err
	}
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(file.Filename)}))
	header.Set("Content-Length", strconv.FormatInt(file.Size, 10))
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(file.Filename))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	header.Set("Content-Type", mimeType)
	for _, id := range ids.FileChunkIds {
		chunk, err := chunks.DownloadFileChunk(ctx, &pb.DownloadFileChunkRequest{FileChunkId: id})
		if err != nil {
			return err
		}
		if chunk.FileChunk == nil {
			return errors.New("file chunk unavailable")
		}
		if _, err := writer.Write(chunk.FileChunk.Data); err != nil {
			return err
		}
	}
	return nil
}
