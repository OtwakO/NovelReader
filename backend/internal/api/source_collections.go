package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
)

type collectionCreateRequest struct {
	Name         string                  `json:"name"`
	URL          string                  `json:"url"`
	SyncInterval booksource.SyncInterval `json:"syncInterval"`
}

type collectionUpdateRequest struct {
	Name         *string                  `json:"name,omitempty"`
	SyncInterval *booksource.SyncInterval `json:"syncInterval,omitempty"`
}

type collectionMutationResponse struct {
	Collection booksource.Collection    `json:"collection"`
	Changes    booksource.ReplaceResult `json:"changes"`
}

func (s *Server) handleListSourceCollections(w http.ResponseWriter, _ *http.Request) {
	collections, err := s.sourceStore.ListCollections()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if collections == nil {
		collections = []booksource.Collection{}
	}
	writeJSON(w, http.StatusOK, collections)
}

func (s *Server) handleCreateUploadCollection(w http.ResponseWriter, r *http.Request) {
	name, filename, document, err := readCollectionUpload(w, r)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	sources, err := booksource.ImportSources(document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if name == "" {
		name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	collection, changes, err := s.sourceStore.CreateCollection(r.Context(), booksource.CreateCollection{
		Name: name, OriginKind: booksource.CollectionOriginUpload, OriginFilename: filename, SyncInterval: booksource.SyncManual,
	}, sources, time.Now())
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, collectionMutationResponse{Collection: collection, Changes: changes})
}

func (s *Server) handleCreateURLCollection(w http.ResponseWriter, r *http.Request) {
	var request collectionCreateRequest
	if err := decodeCollectionJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.SyncInterval == "" {
		request.SyncInterval = booksource.SyncManual
	}
	document, err := s.collectionLoader.Load(r.Context(), request.URL, "", "")
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	sources, err := booksource.ImportSources(document.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	collection, changes, err := s.sourceStore.CreateCollection(r.Context(), booksource.CreateCollection{
		Name: request.Name, OriginKind: booksource.CollectionOriginURL, OriginURL: request.URL, SyncInterval: request.SyncInterval,
		ETag: document.ETag, LastModified: document.LastModified,
	}, sources, time.Now())
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	created, _ := s.sourceStore.GetCollection(collection.ID)
	writeJSON(w, http.StatusCreated, collectionMutationResponse{Collection: *created, Changes: changes})
}

func (s *Server) handleUpdateSourceCollection(w http.ResponseWriter, r *http.Request) {
	var request collectionUpdateRequest
	if err := decodeCollectionJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if request.Name == nil && request.SyncInterval == nil {
		writeError(w, http.StatusBadRequest, "collection update is empty")
		return
	}
	if request.Name != nil {
		if err := s.sourceStore.RenameCollection(r.Context(), id, *request.Name, time.Now()); err != nil {
			writeCollectionError(w, err)
			return
		}
	}
	if request.SyncInterval != nil {
		collection, err := s.sourceStore.GetCollection(id)
		if err != nil || collection == nil {
			writeCollectionError(w, booksource.ErrCollectionNotFound)
			return
		}
		if collection.OriginKind != booksource.CollectionOriginURL && *request.SyncInterval != booksource.SyncManual {
			writeError(w, http.StatusBadRequest, "uploaded collections cannot be scheduled")
			return
		}
		if err := s.sourceStore.UpdateCollectionSchedule(r.Context(), id, *request.SyncInterval, time.Now()); err != nil {
			writeCollectionError(w, err)
			return
		}
	}
	collection, err := s.sourceStore.GetCollection(id)
	if err != nil || collection == nil {
		writeCollectionError(w, booksource.ErrCollectionNotFound)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

func (s *Server) handleReplaceUploadCollection(w http.ResponseWriter, r *http.Request) {
	_, filename, document, err := readCollectionUpload(w, r)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	collection, err := s.sourceStore.GetCollection(r.PathValue("id"))
	if err != nil || collection == nil {
		writeCollectionError(w, booksource.ErrCollectionNotFound)
		return
	}
	if collection.OriginKind != booksource.CollectionOriginUpload {
		writeError(w, http.StatusBadRequest, "URL collections must be synchronized from their configured URL")
		return
	}
	sources, err := booksource.ImportSources(document)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	changes, err := s.sourceStore.ReplaceCollection(r.Context(), collection.ID, sources, filename, "", "", time.Now())
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	collection, _ = s.sourceStore.GetCollection(collection.ID)
	writeJSON(w, http.StatusOK, collectionMutationResponse{Collection: *collection, Changes: changes})
}

func (s *Server) handleSyncSourceCollection(w http.ResponseWriter, r *http.Request) {
	collection, err := s.sourceStore.GetCollection(r.PathValue("id"))
	if err != nil || collection == nil {
		writeCollectionError(w, booksource.ErrCollectionNotFound)
		return
	}
	if collection.OriginKind != booksource.CollectionOriginURL {
		writeError(w, http.StatusBadRequest, "uploaded collections cannot be synchronized")
		return
	}
	document, err := s.collectionLoader.Load(r.Context(), collection.OriginURL, collection.ETag, collection.LastModified)
	if err != nil {
		_ = s.sourceStore.RecordCollectionFailure(r.Context(), collection.ID, err.Error(), time.Now())
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if document.NotModified {
		if err := s.sourceStore.RecordCollectionSuccess(r.Context(), collection.ID, time.Now()); err != nil {
			writeCollectionError(w, err)
			return
		}
		collection, _ = s.sourceStore.GetCollection(collection.ID)
		writeJSON(w, http.StatusOK, collectionMutationResponse{Collection: *collection, Changes: booksource.ReplaceResult{Total: collection.SourceCount, Unchanged: collection.SourceCount}})
		return
	}
	sources, err := booksource.ImportSources(document.Body)
	if err != nil {
		_ = s.sourceStore.RecordCollectionFailure(r.Context(), collection.ID, err.Error(), time.Now())
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	changes, err := s.sourceStore.ReplaceCollection(r.Context(), collection.ID, sources, "", document.ETag, document.LastModified, time.Now())
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	collection, _ = s.sourceStore.GetCollection(collection.ID)
	writeJSON(w, http.StatusOK, collectionMutationResponse{Collection: *collection, Changes: changes})
}

func (s *Server) handleDeleteSourceCollection(w http.ResponseWriter, r *http.Request) {
	if err := s.sourceStore.DeleteCollection(r.Context(), r.PathValue("id")); err != nil {
		writeCollectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readCollectionUpload(w http.ResponseWriter, r *http.Request) (string, string, []byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, booksource.MaxCollectionDocumentBytes+1024*1024)
	if err := r.ParseMultipartForm(booksource.MaxCollectionDocumentBytes); err != nil {
		return "", "", nil, fmt.Errorf("invalid collection upload: %w", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", "", nil, fmt.Errorf("collection file is required")
	}
	defer file.Close()
	document, err := readMultipartDocument(file)
	return strings.TrimSpace(r.FormValue("name")), filepath.Base(header.Filename), document, err
}

func readMultipartDocument(file multipart.File) ([]byte, error) {
	document, err := io.ReadAll(io.LimitReader(file, booksource.MaxCollectionDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(document) > booksource.MaxCollectionDocumentBytes {
		return nil, fmt.Errorf("collection document exceeds 50 MiB")
	}
	return document, nil
}

func decodeCollectionJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid collection request: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("invalid collection request: trailing JSON")
	}
	return nil
}

func writeCollectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, booksource.ErrCollectionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, booksource.ErrCollectionConflict), errors.Is(err, booksource.ErrCollectionNameUsed):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
