// Copyright 2018 Shift Devices AG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

/*
#ifndef BACKEND_H
#define BACKEND_H
#include <string.h>
#include <stdint.h>

typedef void (*pushNotificationsCallback) (const char*);
static void pushNotify(pushNotificationsCallback f, const char* msg) {
    f(msg);
}

typedef void (*responseCallback) (int, const char*);
static void respond(responseCallback f, int queryID, const char* msg) {
    f(queryID, msg);
}

typedef void (*notifyUserCallback) (const char*);
static void notifyUser(notifyUserCallback f, const char* msg) {
    f(msg);
}
#endif
*/
import "C"

import (
	"bytes"
	"flag"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/digitalbitbox/bitbox-wallet-app/backend"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/arguments"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/devices/usb"
	backendHandlers "github.com/digitalbitbox/bitbox-wallet-app/backend/handlers"
	"github.com/digitalbitbox/bitbox-wallet-app/util/config"
	"github.com/digitalbitbox/bitbox-wallet-app/util/errp"
	"github.com/digitalbitbox/bitbox-wallet-app/util/jsonp"
	"github.com/digitalbitbox/bitbox-wallet-app/util/logging"
	"github.com/digitalbitbox/bitbox-wallet-app/util/random"
)

var handlers *backendHandlers.Handlers
var responseCallback C.responseCallback
var token string

type response struct {
	Body bytes.Buffer
}

func (r *response) Header() http.Header {
	// Not needed.
	return http.Header{}
}

func (r *response) Write(buf []byte) (int, error) {
	r.Body.Write(buf)
	return len(buf), nil
}

func (r *response) WriteHeader(int) {
	// Not needed.
}

//export backendCall
func backendCall(queryID C.int, s *C.char) {
	if handlers == nil {
		return
	}
	query := map[string]string{}
	jsonp.MustUnmarshal([]byte(C.GoString(s)), &query)
	if query["method"] != "POST" && query["method"] != "GET" {
		panic(errp.Newf("method must be POST or GET, got: %s", query["method"]))
	}
	go func() {
		defer func() {
			// recover from all panics and log error before panicking again
			if r := recover(); r != nil {
				logging.Get().WithGroup("server").WithField("panic", true).Errorf("%v\n%s", r, string(debug.Stack()))
			}
		}()

		resp := &response{}
		request, err := http.NewRequest(query["method"], "/api/"+query["endpoint"], strings.NewReader(query["body"]))
		if err != nil {
			panic(errp.WithStack(err))
		}
		request.Header.Set("Authorization", "Basic "+token)
		handlers.Router.ServeHTTP(resp, request)
		responseBytes := resp.Body.Bytes()
		C.respond(responseCallback, queryID, C.CString(string(responseBytes)))
	}()
}

// qtEnvironment implements backend.Environment
type qtEnvironment struct {
	notifyUser func(string)
}

// NotifyUser implements backend.Environment
func (env qtEnvironment) NotifyUser(text string) {
	env.notifyUser(text)
}

// DeviceInfos implements backend.Environment
func (env qtEnvironment) DeviceInfos() []usb.DeviceInfo {
	return usb.DeviceInfos()
}

//export serve
func serve(
	pushNotificationsCallback C.pushNotificationsCallback,
	theResponseCallback C.responseCallback,
	notifyUserCallback C.notifyUserCallback,
) {
	responseCallback = theResponseCallback

	// workaround: this flag is parsed by qtwebengine, but flag.Parse() quits the app on
	// unrecognized flags
	// _ = flag.Int("remote-debugging-port", 0, "")
	testnet := flag.Bool("testnet", false, "activate testnets")
	flag.Parse()
	log := logging.Get().WithGroup("server")
	log.Info("--------------- Started application --------------")
	log.WithField("goos", runtime.GOOS).WithField("goarch", runtime.GOARCH).WithField("version", backend.Version).Info("environment")
	theBackend, err := backend.NewBackend(arguments.NewArguments(
		config.AppDir(), *testnet, false, false, false, false),
		&qtEnvironment{
			notifyUser: func(text string) {
				C.notifyUser(notifyUserCallback, C.CString(text))
			},
		},
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to create backend")
	}

	token, err = random.HexString(16)
	if err != nil {
		log.WithError(err).Fatal("Failed to generate random string")
	}

	events := theBackend.Events()
	go func() {
		for {
			C.pushNotify(pushNotificationsCallback, C.CString(string(jsonp.MustMarshal(<-events))))
		}
	}()
	// the port is unused in the Qt app, as we bridge directly without a server.
	const port = -1
	handlers = backendHandlers.NewHandlers(theBackend, backendHandlers.NewConnectionData(port, token))
}

// Don't remove - needed for the C compilation.
func main() {
}
