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
	"os"
	"strconv"
	"strings"

	"github.com/nholuongut/dapr/tests/apps/utils"

	"github.com/gorilla/mux"
)

const (
	daprBaseURL        = "http://localhost:%d//v1.0" // Using "//" to repro regression.
	daprActorMethodURL = daprBaseURL + "/actors/%s/%s/method/%s"
	defaultActorTypes  = "actor1,actor2"        // Actor type must be unique per test app.
	actorTypesEnvName  = "TEST_APP_ACTOR_TYPES" // To set to change actor types.

	actorIdleTimeout        = "5s" // Short idle timeout.
	drainOngoingCallTimeout = "1s"
	drainRebalancedActors   = true
)

var (
	appPort      = 3000
	daprHTTPPort = 3500
)

func init() {
	p := os.Getenv("DAPR_HTTP_PORT")
	if p != "" && p != "0" {
		daprHTTPPort, _ = strconv.Atoi(p)
	}
	p = os.Getenv("PORT")
	if p != "" && p != "0" {
		appPort, _ = strconv.Atoi(p)
	}
}

type callRequest struct {
	ActorType       string `json:"actorType"`
	ActorID         string `json:"actorId"`
	Method          string `json:"method"`
	RemoteActorID   string `json:"remoteId,omitempty"`
	RemoteActorType string `json:"remoteType,omitempty"`
}

type daprConfig struct {
	Entities                []string `json:"entities,omitempty"`
	ActorIdleTimeout        string   `json:"actorIdleTimeout,omitempty"`
	DrainOngoingCallTimeout string   `json:"drainOngoingCallTimeout,omitempty"`
	DrainRebalancedActors   bool     `json:"drainRebalancedActors,omitempty"`
}

var daprConfigResponse = daprConfig{
	getActorTypes(),
	actorIdleTimeout,
	drainOngoingCallTimeout,
	drainRebalancedActors,
}

func getActorTypes() []string {
	actorTypes := os.Getenv(actorTypesEnvName)
	if actorTypes == "" {
		return strings.Split(defaultActorTypes, ",")
	}

	return strings.Split(actorTypes, ",")
}

func parseCallRequest(r *http.Request) (callRequest, []byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Could not read request body: %v", err)
		return callRequest{}, body, err
	}

	var request callRequest
	json.Unmarshal(body, &request)
	return request, body, nil
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// This method is required for actor registration (provides supported types).
func configHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Processing dapr request for %s", r.URL.RequestURI())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(daprConfigResponse)
}

func callActorMethod(w http.ResponseWriter, r *http.Request) {
	log.Println("callActorMethod is called")

	request, body, err := parseCallRequest(r)
	if err != nil {
		log.Printf("Could not parse request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	invokeURL := fmt.Sprintf(daprActorMethodURL, daprHTTPPort, request.ActorType, request.ActorID, request.Method)
	log.Printf("Calling actor with: %s", invokeURL)

	resp, err := http.Post(invokeURL, "application/json", bytes.NewBuffer(body)) //nolint:gosec
	if resp != nil {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Resp: %s", string(respBody))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}
	if err != nil {
		log.Printf("Failed to call actor: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func logCall(w http.ResponseWriter, r *http.Request) {
	log.Printf("logCall is called")

	actorType := mux.Vars(r)["actorType"]
	actorID := mux.Vars(r)["actorId"]

	resp := fmt.Sprintf("Log call with - actorType: %s, actorId: %s", actorType, actorID)
	log.Println(resp)
	w.Write([]byte(resp))
}

func xDaprErrorResponseHeader(w http.ResponseWriter, r *http.Request) {
	log.Printf("xDaprErrorResponseHeader is called")

	actorType := mux.Vars(r)["actorType"]
	actorID := mux.Vars(r)["actorId"]

	resp := fmt.Sprintf("x-DaprErrorResponseHeader call with - actorType: %s, actorId: %s", actorType, actorID)
	log.Println(resp)
	w.Header().Add("x-DaprErrorResponseHeader", "Simulated error")
	w.Write([]byte(resp))
}

func callDifferentActor(w http.ResponseWriter, r *http.Request) {
	log.Println("callDifferentActor is called")

	request, _, err := parseCallRequest(r)
	if err != nil {
		log.Printf("Could not parse request body: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	invokeURL := fmt.Sprintf(daprActorMethodURL, daprHTTPPort, request.RemoteActorType, request.RemoteActorID, "logCall")
	log.Printf("Calling remote actor with: %s", invokeURL)

	resp, err := http.Post(invokeURL, "application/json", bytes.NewBuffer([]byte{})) //nolint:gosec
	if resp != nil {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Resp: %s", string(respBody))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}
	if err != nil {
		log.Printf("Failed to call remote actor: %s\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func deactivateActorHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Deactivated actor: %s", r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// indexHandler is the handler for root path.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("indexHandler is called")

	w.WriteHeader(http.StatusOK)
}

// appRouter initializes restful api router.
func appRouter() http.Handler {
	router := mux.NewRouter().StrictSlash(true)

	// Log requests and their processing time
	router.Use(utils.LoggerMiddleware)

	router.HandleFunc("/", indexHandler).Methods("GET")
	// Actor methods are individually bound so we can experiment with missing messages
	router.HandleFunc("/actors/{actorType}/{actorId}/method/logCall", logCall).Methods("POST", "PUT")
	router.HandleFunc("/actors/{actorType}/{actorId}/method/xDaprErrorResponseHeader", xDaprErrorResponseHeader).Methods("POST", "PUT")
	router.HandleFunc("/actors/{actorType}/{actorId}/method/callDifferentActor", callDifferentActor).Methods("POST", "PUT")
	router.HandleFunc("/actors/{actorType}/{id}", deactivateActorHandler).Methods("POST", "DELETE")
	router.HandleFunc("/dapr/config", configHandler).Methods("GET")
	router.HandleFunc("/healthz", healthzHandler).Methods("GET")
	router.HandleFunc("/test/callActorMethod", callActorMethod).Methods("POST")

	router.Use(mux.CORSMethodMiddleware(router))

	return router
}

func main() {
	log.Printf("Actor Invocation App - listening on http://localhost:%d", appPort)
	utils.StartServer(appPort, appRouter, true, false)
}
