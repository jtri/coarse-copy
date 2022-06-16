package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

var logger = log.New(os.Stdout, "[coarse-copy] ", log.LstdFlags)

func main() {
	db := mustGetDBSession()
	server := newServer(logger, db)

	if err := migrate(db); err != nil {
		logger.Println("failed to migrate database", err)
	}

	mux := mux.NewRouter()
	mux.HandleFunc(getQuestionPath, WithError(server.getQuestion)).Methods(http.MethodGet)
	mux.HandleFunc(getQuestionsPath, WithError(server.getQuestions)).Methods(http.MethodGet)
	mux.HandleFunc(createQuestionPath, WithError(server.createQuestion)).Methods(http.MethodPost)
	mux.HandleFunc(updateQuestionPath, WithError(server.updateQuestion)).Methods(http.MethodPut)
	mux.HandleFunc(deleteQuestionPath, WithError(server.deleteQuestion)).Methods(http.MethodDelete)
	mux.HandleFunc(upsertQuestionPath, WithError(server.upsertQuestions)).Methods(http.MethodPut)

	srv := &http.Server{
		Handler:      mux,
		Addr:         ":8888",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
	}

	logger.Println("listening on ", srv.Addr)
	logger.Println("listen and serve", srv.ListenAndServe())
}

type server struct {
	logger *log.Logger
	store  *store
}

func newServer(logger *log.Logger, db *gorm.DB) *server {
	return &server{logger: logger, store: &store{db}}

}

type handleFunc func(w http.ResponseWriter, r *http.Request) error

func WithError(h handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Internal Server Error`))
			fmt.Println("something went wrong", err)
		}
	}
}

type getQuestionRequest struct{}
type getQuestionResponse struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

const getQuestionPath = `/api/v1/questions/{id}`

func (s *server) getQuestion(w http.ResponseWriter, r *http.Request) error {
	input := getQuestionRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	id := mux.Vars(r)["id"]

	q, err := s.store.getQuestion(id)
	if err != nil {
		return err
	}

	output := getQuestionResponse{ID: q.ID, Text: q.Text}
	return writeJSONResponse(w, http.StatusOK, &output)
}

type getQuestionsRequest struct{}
type getQuestionsResponse struct {
	Questions []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"questions"`
}

const getQuestionsPath = `/api/v1/questions`

func (s *server) getQuestions(w http.ResponseWriter, r *http.Request) error {
	input := getQuestionsRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	qs, err := s.store.getQuestions()
	if err != nil {
		return err
	}

	output := getQuestionsResponse{Questions: []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}{}}
	for _, q := range qs {
		output.Questions = append(output.Questions, struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		}{
			ID:   q.ID,
			Text: q.Text,
		})
	}
	return writeJSONResponse(w, http.StatusOK, &output)
}

type createQuestionRequest struct {
	Text string `json:"text"`
}
type createQuestionResponse struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

const createQuestionPath = `/api/v1/questions`

func (s *server) createQuestion(w http.ResponseWriter, r *http.Request) error {
	input := createQuestionRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	id := uuid.NewString()

	err := s.store.createQuestion(id, input.Text)
	if err != nil {
		return err
	}

	output := createQuestionResponse{ID: id, Text: input.Text}
	return writeJSONResponse(w, http.StatusOK, &output)
}

type updateQuestionRequest struct {
	Text string `json:"text"`
}
type updateQuestionResponse struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

const updateQuestionPath = `/api/v1/questions/{id}`

func (s *server) updateQuestion(w http.ResponseWriter, r *http.Request) error {
	input := updateQuestionRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	id := mux.Vars(r)["id"]

	err := s.store.updateQuestion(id, input.Text)
	if err != nil {
		return err
	}

	output := updateQuestionResponse{ID: id, Text: input.Text}
	return writeJSONResponse(w, http.StatusOK, &output)
}

type deleteQuestionRequest struct{}
type deleteQuestionResponse struct{}

const deleteQuestionPath = `/api/v1/questions/{id}`

func (s *server) deleteQuestion(w http.ResponseWriter, r *http.Request) error {
	input := deleteQuestionRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	id := mux.Vars(r)["id"]

	err := s.store.deleteQuestion(id)
	if err != nil {
		return err
	}

	output := deleteQuestionResponse{}
	return writeJSONResponse(w, http.StatusNoContent, &output)
}

type upsertQuestionRequest struct {
	Questions []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"questions"`
}
type upsertQuestionResponse struct {
	Questions []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"questions"`
}

const upsertQuestionPath = `/api/v1/questions`

func (s *server) upsertQuestions(w http.ResponseWriter, r *http.Request) error {
	input := upsertQuestionRequest{}
	if err := readJSONRequest(r.Body, &input); err != nil {
		return err
	}

	qs := []*Question{}
	for _, q := range input.Questions {
		qs = append(qs, &Question{ID: q.ID, Text: q.Text})
	}

	err := s.store.upsertQuestion(qs)
	if err != nil {
		return err
	}

	output := upsertQuestionResponse{Questions: input.Questions}
	return writeJSONResponse(w, http.StatusOK, &output)
}

// readJSONRequest reads the body into the dest or returns an error if any occurred
func readJSONRequest(body io.Reader, dest interface{}) error {
	if body == nil {
		return nil
	}

	bs, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	if len(bs) == 0 {
		return nil
	}

	return json.Unmarshal(bs, dest)
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, in interface{}) error {
	bs, err := json.Marshal(in)
	if err != nil {
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(bs)
	return nil
}
