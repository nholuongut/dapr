/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nholuongut/dapr/tests/apps/utils"

	"github.com/gorilla/mux"
)

const (
	appPort              = 3000
	daprV1URL            = "http://localhost:3500/v1.0"
	actorMethodURLFormat = daprV1URL + "/actors/%s/%s/method/%s"

	registeredActorType     = "testactor" // Actor type must be unique per test app.
	actorIdleTimeout        = "5s"        // Short idle timeout.
	drainOngoingCallTimeout = "1s"
	drainRebalancedActors   = true
)

type daprActor struct {
	actorType string
	id        string
	value     interface{}
}

// represents a response for the APIs in this app.
type actorLogEntry struct {
	Action    string `json:"action,omitempty"`
	ActorType string `json:"actorType,omitempty"`
	ActorID   string `json:"actorId,omitempty"`
	Timestamp int    `json:"timestamp,omitempty"`
}

type daprConfig struct {
	Entities                []string `json:"entities,omitempty"`
	ActorIdleTimeout        string   `json:"actorIdleTimeout,omitempty"`
	DrainOngoingCallTimeout string   `json:"drainOngoingCallTimeout,omitempty"`
	DrainRebalancedActors   bool     `json:"drainRebalancedActors,omitempty"`
}

var daprConfigResponse = daprConfig{
	[]string{registeredActorType},
	actorIdleTimeout,
	drainOngoingCallTimeout,
	drainRebalancedActors,
}

var (
	actorLogs      = []actorLogEntry{}
	actorLogsMutex = &sync.Mutex{}
)

var actors sync.Map

func appendActorLog(logEntry actorLogEntry) {
	actorLogsMutex.Lock()
	defer actorLogsMutex.Unlock()
	actorLogs = append(actorLogs, logEntry)
}

func getActorLogs() []actorLogEntry {
	return actorLogs
}

func createActorID(actorType string, id string) string {
	return actorType + "." + id
}

// indexHandler is the handler for root path
func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("indexHandler is called")

	w.WriteHeader(http.StatusOK)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing dapr request for %s", r.URL.RequestURI())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(getActorLogs())
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing dapr request for %s", r.URL.RequestURI())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(daprConfigResponse)
}

func actorMethodHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing actor method request for %s", r.URL.RequestURI())

	actorType := mux.Vars(r)["actorType"]
	id := mux.Vars(r)["id"]
	method := mux.Vars(r)["method"]

	actorID := createActorID(actorType, id)
	log.Printf("storing actorID %s\n", actorID)

	actors.Store(actorID, daprActor{
		actorType: actorType,
		id:        actorID,
		value:     nil,
	})
	appendActorLog(actorLogEntry{
		Action:    method,
		ActorType: actorType,
		ActorID:   id,
		Timestamp: epoch(),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

//nolint:forbidigo
func deactivateActorHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing %s actor request for %s", r.Method, r.URL.RequestURI())

	actorType := mux.Vars(r)["actorType"]
	id := mux.Vars(r)["id"]

	if actorType != registeredActorType {
		log.Printf("Unknown actor type: %s", actorType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	actorID := createActorID(actorType, id)
	fmt.Printf("actorID is %s\n", actorID)

	action := ""
	_, ok := actors.Load(actorID)
	log.Printf("loading returned:%t\n", ok)

	if ok && r.Method == http.MethodDelete {
		action = "deactivation"
		actors.Delete(actorID)
	}

	appendActorLog(actorLogEntry{
		Action:    action,
		ActorType: actorType,
		ActorID:   id,
		Timestamp: epoch(),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// calls Dapr's Actor method: simulating actor client call.
//
//nolint:gosec
func testCallActorHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing %s test request for %s", r.Method, r.URL.RequestURI())

	actorType := mux.Vars(r)["actorType"]
	id := mux.Vars(r)["id"]
	method := mux.Vars(r)["method"]

	invokeURL := fmt.Sprintf(actorMethodURLFormat, actorType, id, method)
	log.Printf("Invoking %s", invokeURL)

	res, err := http.Post(invokeURL, "application/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		log.Printf("Could not test actor: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Could not read actor's test response: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(body)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// epoch returns the current unix epoch timestamp
func epoch() int {
	return (int)(time.Now().UTC().UnixNano() / 1000000)
}

// appRouter initializes restful api router
func appRouter() http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	// Log requests and their processing time
	router.Use(utils.LoggerMiddleware)

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/dapr/config", configHandler).Methods("GET")
	router.HandleFunc("/actors/{actorType}/{id}/method/{method}", actorMethodHandler).Methods("PUT")
	router.HandleFunc("/actors/{actorType}/{id}", deactivateActorHandler).Methods("POST", "DELETE")
	router.HandleFunc("/test/{actorType}/{id}/method/{method}", testCallActorHandler).Methods("POST")
	router.HandleFunc("/test/logs", logsHandler).Methods("GET")
	router.HandleFunc("/healthz", healthzHandler).Methods("GET")

	router.Use(mux.CORSMethodMiddleware(router))

	return router
}

func main() {
	log.Printf("Actor App - listening on http://localhost:%d", appPort)
	utils.StartServer(appPort, appRouter, true, false)
}
