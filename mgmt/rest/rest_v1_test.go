// +build medium

/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2015 Intel Corporation

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

package rest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	log "github.com/Sirupsen/logrus"
	"github.com/intelsdi-x/snap/core/cdata"
	"github.com/intelsdi-x/snap/core/ctypes"
	"github.com/intelsdi-x/snap/mgmt/rest/fixtures"
)

var (
	LOG_LEVEL         = log.WarnLevel
	SNAP_PATH         = os.ExpandEnv(os.Getenv("SNAP_PATH"))
	MOCK_PLUGIN_PATH1 = SNAP_PATH + "/plugin/snap-plugin-collector-mock1"
	MOCK_TASK_PATH1   = SNAP_PATH + "/tasks/snap-task-collector-mock1"
)

type restAPIInstance struct {
	port   int
	server *Server
}

func startV1API(cfg *mockConfig) *restAPIInstance {
	log.SetLevel(LOG_LEVEL)
	r, _ := New(cfg.RestAPI)
	mockMetricManager := &fixtures.MockManagesMetrics{}
	mockTaskManager := &fixtures.MockTaskManager{}
	mockConfigManager := &fixtures.MockConfigManager{}
	mockTribeManager := &fixtures.MockTribeManager{}
	r.BindMetricManager(mockMetricManager)
	r.BindTaskManager(mockTaskManager)
	r.BindConfigManager(mockConfigManager)
	r.BindTribeManager(mockTribeManager)
	go func(ch <-chan error) {
		// Block on the error channel. Will return exit status 1 for an error or
		// just return if the channel closes.
		err, ok := <-ch
		if !ok {
			return
		}
		log.Fatal(err)
	}(r.Err())
	r.SetAddress("127.0.0.1:0")
	r.Start()
	return &restAPIInstance{
		port:   r.Port(),
		server: r,
	}
}

func TestV1(t *testing.T) {
	r := startV1API(getDefaultMockConfig())
	Convey("Test REST API V1", t, func() {

		//////////TEST-PLUGIN-ROUTES/////////////////
		Convey("Get plugins - v1/plugins", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/plugins", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_PLUGINS_RESPONSE, r.port, r.port,
					r.port, r.port, r.port, r.port),
				ShouldResemble,
				string(body))
		})
		Convey("Get plugins - v1/plugins/:type", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/plugins/collector", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_PLUGINS_RESPONSE_TYPE, r.port, r.port),
				ShouldResemble,
				string(body))
		})
		Convey("Get plugins - v1/plugins/:type:name", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/plugins/publisher/bar", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_PLUGINS_RESPONSE_TYPE_NAME, r.port),
				ShouldResemble,
				string(body))
		})
		Convey("Get plugins - v1/plugins/:type:name:version", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/plugins/publisher/bar/3", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_PLUGINS_RESPONSE_TYPE_NAME_VERSION, r.port),
				ShouldResemble,
				string(body))
		})

		Convey("Post plugins - v1/plugins/:type:name", func(c C) {
			f, err := os.Open(MOCK_PLUGIN_PATH1)
			defer f.Close()
			So(err, ShouldBeNil)

			// We create a pipe so that we can write the file in multipart
			// format and read it in to the body of the http request
			reader, writer := io.Pipe()
			mwriter := multipart.NewWriter(writer)
			bufin := bufio.NewReader(f)

			// A go routine is needed since we must write the multipart file
			// to the pipe so we can read from it in the http call
			go func() {
				part, err := mwriter.CreateFormFile("snap-plugins", "mock")
				c.So(err, ShouldBeNil)
				bufin.WriteTo(part)
				mwriter.Close()
				writer.Close()
			}()

			resp1, err1 := http.Post(
				fmt.Sprintf("http://localhost:%d/v1/plugins", r.port),
				mwriter.FormDataContentType(), reader)
			So(err1, ShouldBeNil)
			So(resp1.StatusCode, ShouldEqual, 201)
		})

		Convey("Delete plugins - v1/plugins/:type:name:version", func() {
			c := &http.Client{}
			pluginName := "foo"
			pluginType := "collector"
			pluginVersion := 2
			req, err := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/v1/plugins/%s/%s/%d",
					r.port,
					pluginType,
					pluginName,
					pluginVersion),
				bytes.NewReader([]byte{}))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.UNLOAD_PLUGIN),
				ShouldResemble,
				string(body))
		})

		Convey("Get plugin config items - v1/plugins/:type/:name/:version/config", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/plugins/publisher/bar/3/config", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_PLUGIN_CONFIG_ITEM),
				ShouldResemble,
				string(body))
		})

		Convey("Put plugins - v1/plugins/:type/:name/:version/config", func() {
			c := &http.Client{}
			pluginName := "foo"
			pluginType := "collector"
			pluginVersion := 2
			cd := cdata.NewNode()
			cd.AddItem("user", ctypes.ConfigValueStr{Value: "Jane"})
			body, err := cd.MarshalJSON()
			So(err, ShouldBeNil)

			req, err := http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("http://localhost:%d/v1/plugins/%s/%s/%d/config",
					r.port,
					pluginType,
					pluginName,
					pluginVersion),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.PUT_PLUGIN_CONFIG_ITEM),
				ShouldResemble,
				string(body))

		})

		Convey("Delete Plugin Config Item - /v1/plugins/:type/:name/:version/config", func() {
			c := &http.Client{}
			pluginName := "foo"
			pluginType := "collector"
			pluginVersion := 2
			cd := []string{"foo"}
			body, err := json.Marshal(cd)
			So(err, ShouldBeNil)
			req, err := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/v1/plugins/%s/%s/%d/config",
					r.port,
					pluginType,
					pluginName,
					pluginVersion),
				bytes.NewReader(body))

			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.DELETE_PLUGIN_CONFIG_ITEM),
				ShouldResemble,
				string(body))

		})

		//////////TEST-METRIC-ROUTES/////////////////

		Convey("Get metric items - v1/metrics", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/metrics", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			resp1, err := url.QueryUnescape(string(body))
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_METRICS_RESPONSE, r.port),
				ShouldResemble,
				resp1)
		})

		Convey("Get metric items - v1/metrics/*namespace", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/metrics/*namespace", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			resp1, err := url.QueryUnescape(string(body))
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_METRICS_RESPONSE, r.port),
				ShouldResemble,
				resp1)
		})

		//////////TEST-TASK-ROUTES/////////////////

		Convey("Get tasks - v1/tasks", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tasks", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_TASKS_RESPONSE, r.port, r.port),
				ShouldResemble,
				string(body))
		})

		Convey("Get task - v1/tasks/:id", func() {
			taskID := "1234"
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tasks/:%s", r.port, taskID))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_TASK_RESPONSE, r.port),
				ShouldResemble,
				string(body))
		})

		Convey("Get tasks - v1/tasks/:id/watch", func() {
			taskID := "1234"
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tasks/:%s/watch", r.port, taskID))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
		})

		Convey("Post tasks - v1/tasks", func() {
			reader := strings.NewReader(fixtures.TASK)
			resp, err := http.Post(fmt.Sprintf("http://localhost:%d/v1/tasks", r.port),
				http.DetectContentType([]byte(fixtures.TASK)), reader)
			So(err, ShouldBeNil)
			So(resp, ShouldNotBeEmpty)
			So(resp.StatusCode, ShouldEqual, 201)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.POST_TASK, r.port),
				ShouldResemble,
				string(body))
		})

		Convey("Put tasks - v1/tasks/:id/start", func() {
			c := &http.Client{}
			taskID := "MockTask1234"
			cd := cdata.NewNode()
			cd.AddItem("user", ctypes.ConfigValueStr{Value: "Kelly"})
			body, err := cd.MarshalJSON()
			So(err, ShouldBeNil)

			req, err := http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("http://localhost:%d/v1/tasks/%s/start", r.port, taskID),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.PUT_TASK_ID_START),
				ShouldResemble,
				string(body))
		})

		Convey("Put tasks - v1/tasks/:id/stop", func() {
			c := &http.Client{}
			taskID := "MockTask1234"
			cd := cdata.NewNode()
			cd.AddItem("user", ctypes.ConfigValueStr{Value: "Kelly"})
			body, err := cd.MarshalJSON()
			So(err, ShouldBeNil)

			req, err := http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("http://localhost:%d/v1/tasks/%s/stop", r.port, taskID),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.PUT_TASK_ID_STOP),
				ShouldResemble,
				string(body))
		})

		Convey("Put tasks - v1/tasks/:id/enable", func() {
			c := &http.Client{}
			taskID := "MockTask1234"
			cd := cdata.NewNode()
			cd.AddItem("user", ctypes.ConfigValueStr{Value: "Kelly"})
			body, err := cd.MarshalJSON()
			So(err, ShouldBeNil)

			req, err := http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("http://localhost:%d/v1/tasks/%s/enable", r.port, taskID),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.PUT_TASK_ID_ENABLE),
				ShouldResemble,
				string(body))
		})

		Convey("Delete tasks - V1/tasks/:id", func() {
			c := &http.Client{}
			taskID := "MockTask1234"
			cd := []string{"foo"}
			body, err := json.Marshal(cd)
			So(err, ShouldBeNil)
			req, err := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/v1/tasks/%s",
					r.port,
					taskID),
				bytes.NewReader([]byte{}))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.DELETE_TASK_ID),
				ShouldResemble,
				string(body))
		})

		//////////TEST-TRIBE-ROUTES/////////////////

		Convey("Get tribe agreements - v1/tribe/agreements", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tribe/agreements", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_TRIBE_AGREEMENTS),
				ShouldResemble,
				string(body))
		})

		Convey("Post tribe agreements - /v1/tribe/agreements", func() {
			agreement := "{\"Name\": \"Agree2\"}"
			reader := strings.NewReader(agreement)
			resp, err := http.Post(fmt.Sprintf("http://localhost:%d/v1/tribe/agreements", r.port),
				http.DetectContentType([]byte(agreement)), reader)
			So(err, ShouldBeNil)
			So(resp, ShouldNotBeEmpty)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.POST_TRIBE_AGREEMENT),
				ShouldResemble,
				string(body))
		})

		Convey("Get tribe agreements - v1/tribe/agreements/:name", func() {
			tribeName := "Agree1"
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tribe/agreements/%s", r.port, tribeName))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_TRIBE_AGREEMENTS_NAME),
				ShouldResemble,
				string(body))
		})

		Convey("Get tribe members - v1/tribe/members", func() {
			resp, err := http.Get(
				fmt.Sprintf("http://localhost:%d/v1/tribe/members", r.port))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, 200)
			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.GET_TRIBE_MEMBERS),
				ShouldResemble,
				string(body))
		})

		Convey("Delete tribe agreements - v1/tribe/agreements/:name", func() {
			c := &http.Client{}
			tribeName := "Agree1"
			cd := []string{"foo"}
			body, err := json.Marshal(cd)
			So(err, ShouldBeNil)
			req, err := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/v1/tribe/agreements/%s",
					r.port,
					tribeName),
				bytes.NewReader([]byte{}))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.DELETE_TRIBE_AGREEMENTS_NAME),
				ShouldResemble,
				string(body))
		})

		Convey("Delete tribe agreements - v1/tribe/agreements/:name/leave", func() {
			c := &http.Client{}
			tribeName := "Agree1"
			cd := map[string]string{"Apple": "a", "Ball": "b", "Cat": "c"}
			body, err := json.Marshal(cd)
			So(err, ShouldBeNil)
			req, err := http.NewRequest(
				http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/v1/tribe/agreements/%s/leave",
					r.port,
					tribeName),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.DELETE_TRIBE_AGREEMENTS_NAME_LEAVE),
				ShouldResemble,
				string(body))
		})

		Convey("Put tribe agreements - v1/tribe/agreements/:name/join", func() {
			c := &http.Client{}
			tribeName := "Agree1"
			cd := cdata.NewNode()
			cd.AddItem("user", ctypes.ConfigValueStr{Value: "Kelly"})
			body, err := cd.MarshalJSON()
			So(err, ShouldBeNil)

			req, err := http.NewRequest(
				http.MethodPut,
				fmt.Sprintf("http://localhost:%d/v1/tribe/agreements/%s/join", r.port, tribeName),
				bytes.NewReader(body))
			So(err, ShouldBeNil)
			resp, err := c.Do(req)
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			body, err = ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(
				fmt.Sprintf(fixtures.PUT_TRIBE_AGREEMENTS_NAME_JOIN),
				ShouldResemble,
				string(body))

		})
	})
}
