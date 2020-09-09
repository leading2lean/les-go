/*
This is an example program for how to use the L2L Dispatch API to read and write data in the Dispatch system.

This code is written for Golang 1.14+. To use this code, you need to make sure Golang 1.14 is installed.

You can build the executable using:
    $ go build main.go

To run this code, you would then run:
    $ ./main
*/

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// These are the standard datetime string formats that the Dispatch API supports
var API_MINUTE_FORMAT = "2006-01-02 15:04"
var API_SECONDS_FORMAT = "2006-01-02 15:04:05"

var params url.Values
var dr DataResponse
var lr ListResponse
var apikey string
var site string

type ListResponse struct {
	Success bool             `json:"success"`
	Error   string           `json:"error"`
	Data    []DispatchRecord `json:"data"`
}

type DispatchRecord struct {
	Id          int    `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type DataResponse struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	Data    DispatchObj `json:"data"`
}

type DispatchObj struct {
	Id int `json:"id"`
}

func respcheck(resp *http.Response, err error) []byte {
	if err != nil {
		panic(err)
	} else if resp.StatusCode != 200 {
		panic(resp.Status)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return data
}

func datacheck(data []byte, ro *ListResponse) {
	err := json.Unmarshal(data, &ro)
	if err != nil {
		panic(err)
	}
	if ro.Success == false {
		panic(ro.Error)
	}
}

func setParams(p map[string]string) url.Values {
	params = url.Values{}
	params.Add("auth", apikey)
	params.Add("site", site)

	for k, v := range p {
		params.Add(k, v)
	}

	return params
}

func log(debug bool, msg string, data []byte) {
	fmt.Println(msg)
	if debug {
		fmt.Println(string(data) + "\n")
	}
}

func main() {
	var siteData DispatchRecord
	var areaData DispatchRecord
	var lineData DispatchRecord
	var machineData DispatchRecord
	var dispatchtypeData DispatchRecord

	var dbgFlg = flag.Bool("dbg", false, "Print out verbose api output for debugging")
	var serverFlg = flag.String("server", "", "Specify a hostname to use as the server")
	var siteFlg = flag.String("site", "", "Specify the site id to operate against")
	var userFlg = flag.String("user", "", "Specify the username for a user to use in the test")

	// This example has you pass your API key in on the command line. Note that you should not do this in your
	// production code. The API Key MUST be kept secret, and effective secrets management is outside the scope
	// of this document. Make sure you don't hard code your api key into your source code, and usually you should
	// expose it to your production code through an environment variable.
	var apikeyFlg = flag.String("apikey", "", "Specify an API key to use for authentication")

	flag.Parse()

	var dbg = *dbgFlg
	var server = *serverFlg
	var testuser = *userFlg
	site = *siteFlg
	apikey = *apikeyFlg

	if server == "" || site == "" || testuser == "" || apikey == "" {
		flag.PrintDefaults()
		panic("\n\nMissing required command line arguments: server, site, user or apikey")
	}

	baseUrl, err := url.Parse(fmt.Sprintf("https://%s", server))
	if err != nil {
		panic(fmt.Errorf("Malformed URL: %s", err.Error()))
	}

	baseUrl.Path = "api/1.0/sites/"
	params = setParams(map[string]string{"active": "true", "test_site": "true"})
	baseUrl.RawQuery = params.Encode()

	resp, err := http.Get(baseUrl.String())
	data := respcheck(resp, err)
	datacheck(data, &lr)

	if len(lr.Data) > 0 {
		siteData = lr.Data[0]
	}
	if len(lr.Data) != 1 {
		panic(fmt.Errorf("Invalid test site specified"))
	}
	log(dbg, fmt.Sprintf("Using site: %s", siteData.Description), data)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's find an area/line/machine to use
	finished := false
	limit := 2
	offset := 0

	baseUrl.Path = "api/1.0/areas/"
	params = setParams(map[string]string{"active": "true", "limit": "2", "offset": "0"})
	baseUrl.RawQuery = params.Encode()

	for !finished {
		resp, err := http.Get(baseUrl.String())
		data := respcheck(resp, err)
		datacheck(data, &lr)

		if len(lr.Data) <= limit {
			finished = true
			if len(lr.Data) > 0 {
				areaData = lr.Data[len(lr.Data)-1]
			}
		} else {
			offset += len(lr.Data)
			areaData = lr.Data[len(lr.Data)-1]
		}
	}

	if areaData.Id == 0 {
		panic(fmt.Errorf("Couldn't find an active area to use"))
	}
	log(dbg, fmt.Sprintf("Using area: %s", areaData.Code), data)

	// Grab a line in the area we've found that supports production
	baseUrl.Path = "api/1.0/lines/"
	params = setParams(map[string]string{"area_id": strconv.Itoa(areaData.Id), "active": "true"})
	baseUrl.RawQuery = params.Encode()

	resp, err = http.Get(baseUrl.String())
	data = respcheck(resp, err)
	datacheck(data, &lr)

	if len(lr.Data) > 0 {
		lineData = lr.Data[0]
	}
	if lineData.Id == 0 {
		panic(fmt.Errorf("Couldn't find an active line to use"))
	}
	log(dbg, fmt.Sprintf("Using line: %s", lineData.Code), data)

	// Grab a machine on the line
	baseUrl.Path = "api/1.0/machines/"
	params = setParams(map[string]string{"line_id": strconv.Itoa(lineData.Id), "active": "true"})
	baseUrl.RawQuery = params.Encode()

	resp, err = http.Get(baseUrl.String())
	data = respcheck(resp, err)
	datacheck(data, &lr)

	if len(lr.Data) > 0 {
		machineData = lr.Data[0]
	}
	if machineData.Id == 0 {
		panic(fmt.Errorf("Couldn't find an active machine to use"))
	}
	log(dbg, fmt.Sprintf("Using machine: %s", machineData.Code), data)

	// Grab a Dispatch Type we can use
	params = setParams(map[string]string{"active": "true"})
	baseUrl.RawQuery = params.Encode()
	baseUrl.Path = "api/1.0/dispatchtypes/"

	resp, err = http.Get(baseUrl.String())
	data = respcheck(resp, err)
	datacheck(data, &lr)

	dispatchtypeData = lr.Data[0]
	if dispatchtypeData.Id == 0 {
		panic(fmt.Errorf("Couldn't find an active dispatch type to use"))
	}

	log(dbg, fmt.Sprintf("Using dispatch type: %s", dispatchtypeData.Code), nil)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's record a user clocking in to Dispatch to work on our line we found previously
	params = setParams(map[string]string{"linecode": lineData.Code})
	baseUrl.Path = fmt.Sprintf("api/1.0/users/clock_in/%s/", testuser)

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	datacheck(data, &lr)

	log(dbg, "User clocked in", data)

	// Now clock out the user
	baseUrl.Path = fmt.Sprintf("api/1.0/users/clock_out/%s/", testuser)
	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	datacheck(data, &lr)

	log(dbg, "User clocked out", data)

	// We can record a user clockin session in the past by supplying a start and an end parameter. These datetime
	// parameters in the API must be formatted consistently, and must represent the current time in the Site's
	// timezone (NOT UTC) unless otherwise noted in the API documentation.
	now := time.Now()
	start := now.AddDate(0, 0, -7)
	end := start.Add(time.Hour * 8)

	start_parsed := start.Format(API_MINUTE_FORMAT)
	end_parsed := end.Format(API_MINUTE_FORMAT)
	params = setParams(map[string]string{"start": start_parsed, "end": end_parsed, "linecode": lineData.Code})
	baseUrl.Path = fmt.Sprintf("api/1.0/users/clock_in/%s/", testuser)

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	datacheck(data, &lr)

	log(dbg, "Created backdated clock in", data)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's call specific api's for the machine we found. Here we set the machine's cycle count, and then
	// we increment the machine's cycle count.
	params = setParams(map[string]string{"start": start_parsed, "end": end_parsed, "code": machineData.Code, "cyclecount": "832"})
	baseUrl.Path = "api/1.0/machines/set_cycle_count/"

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	datacheck(data, &lr)

	log(dbg, "Set machine cycle count", data)

	// This simulates a high frequency machine where we make so many calls to this we don't care about tracking the
	// lastupdated values for the machine cycle count.
	params = setParams(map[string]string{"start": start_parsed, "end": end_parsed, "code": machineData.Code, "skip_lastupdated": "1", "cyclecount": "5"})
	baseUrl.Path = "api/1.0/machines/increment_cycle_count/"

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	datacheck(data, &lr)

	log(dbg, "Incremented machine cycle count", data)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's create a Dispatch for the machine, to simulate an event that requires intervention
	params = setParams(map[string]string{"start": start_parsed, "end": end_parsed, "dispatchtype": strconv.Itoa(dispatchtypeData.Id),
		"description": "l2lsdk test dispatch", "machine": strconv.Itoa(machineData.Id)})
	baseUrl.Path = "api/1.0/dispatches/open/"

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	err = json.Unmarshal(data, &dr)
	if err != nil {
		panic(err)
	}
	if dr.Success == false {
		panic(dr.Error)
	}

	log(dbg, "Created open Dispatch", data)

	// Now let's close it
	params = setParams(map[string]string{})
	baseUrl.Path = fmt.Sprintf("api/1.0/dispatches/close/%d/", dr.Data.Id)
	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(""))
	data = respcheck(resp, err)
	err = json.Unmarshal(data, &dr)
	if err != nil {
		panic(err)
	}
	if dr.Success == false {
		panic(dr.Error)
	}

	log(dbg, "Closed open Dispatch", data)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's add a Dispatch for the machine that represents an event that already happened and we just want to record it
	now = time.Now()
	reported := now.AddDate(0, 0, -60)
	completed := reported.Add(time.Minute * 34)

	params = setParams(map[string]string{"dispatchtypecode": dispatchtypeData.Code, "description": "l2lsdk test dispatch (already closed)",
		"machinecode": machineData.Code, "reported": reported.Format(API_MINUTE_FORMAT),
		"completed": completed.Format(API_MINUTE_FORMAT)})
	baseUrl.Path = "api/1.0/dispatches/add/"

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	err = json.Unmarshal(data, &dr)
	if err != nil {
		panic(err)
	}
	if dr.Success == false {
		panic(dr.Error)
	}
	log(dbg, "Created backdated Dispatch", data)

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Let's record some production data using the record_details api. This will create a 1 second pitch as we use now
	// both start and end. Typically you should use a real time range for the start and end values.
	params = setParams(map[string]string{"linecode": lineData.Code, "productcode": "testproduct-3",
		"actual": strconv.Itoa(10 + rand.Intn(90)), "scrap": strconv.Itoa(5 + rand.Intn(15)),
		"operator_count": strconv.Itoa(rand.Intn(10)), "start": "now", "end": "now"})
	baseUrl.Path = "api/1.0/pitchdetails/record_details/"

	resp, err = http.Post(baseUrl.String(), "application/x-www-form-urlencoded", bytes.NewBufferString(params.Encode()))
	data = respcheck(resp, err)
	err = json.Unmarshal(data, &dr)
	if err != nil {
		panic(err)
	}
	if dr.Success == false {
		panic(dr.Error)
	}

	log(dbg, "record_details Pitch details", data)

	// Let's get the production reporting data for our line
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end = now.AddDate(0, 0, 1)

	params = setParams(map[string]string{"start": start.Format(API_SECONDS_FORMAT), "end": end.Format(API_SECONDS_FORMAT), "linecode": lineData.Code, "productcode": "testproduct-3", "show_products": "true"})
	baseUrl.RawQuery = params.Encode()
	baseUrl.Path = "api/1.0/pitchdetails/record_details/"

	resp, err = http.Get(baseUrl.String())
	data = respcheck(resp, err)
	err = json.Unmarshal(data, &dr)
	if err != nil {
		panic(err)
	}
	if dr.Success == false {
		panic(dr.Error)
	}

	log(dbg, "Retrieved Daily summary for line", data)
}
