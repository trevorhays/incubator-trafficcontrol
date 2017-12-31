package main

/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	tclog "github.com/apache/incubator-trafficcontrol/lib/go-log"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/api"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/auth"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/cdn"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/phys_location"
	"github.com/basho/riak-go-client"
)

var Authenticated = true
var NoAuth = false

func handlerToFunc(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

// Routes returns the routes, and a catchall route for when no route matches.
func Routes(d ServerData) ([]Route, http.Handler, error) {
	proxyHandler := rootHandler(d)

	routes := []Route{
		//ASNs
		{1.2, http.MethodGet, `asns/?(\.json)?$`, ASNsHandler(d.DB), ASNsPrivLevel, Authenticated, nil},
		//CDNs
		{1.2, http.MethodGet, `cdns/?(\.json)?$`, cdn.GetHandler(d.DB), cdn.CDNsPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `cdns/{id}$`, cdn.GetHandler(d.DB), cdn.CDNsPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `cdns/{name}/configs/monitoring(\.json)?$`, monitoringHandler(d.DB), MonitoringPrivLevel, Authenticated, nil},
		//CDN generic handlers:
		{1.3, http.MethodPut, `cdns/{id}$`, api.UpdateHandler(cdn.GetRefType(), d.DB), cdn.CDNsPrivLevel, Authenticated, nil},
		{1.3, http.MethodPost, `cdns/?$`, api.CreateHandler(cdn.GetRefType(), d.DB), cdn.CDNsPrivLevel, Authenticated, nil},
		{1.3, http.MethodDelete, `cdns/{id}$`, api.DeleteHandler(cdn.GetRefType(), d.DB), cdn.CDNsPrivLevel, Authenticated, nil},

		// Delivery services
		{1.3, http.MethodGet, `deliveryservices/{xmlID}/urisignkeys$`, getURIsignkeysHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		{1.3, http.MethodPost, `deliveryservices/{xmlID}/urisignkeys$`, saveDeliveryServiceURIKeysHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		{1.3, http.MethodPut, `deliveryservices/{xmlID}/urisignkeys$`, saveDeliveryServiceURIKeysHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		{1.3, http.MethodDelete, `deliveryservices/{xmlID}/urisignkeys$`, removeDeliveryServiceURIKeysHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		//Divisions
		{1.2, http.MethodGet, `divisions/?(\.json)?$`, divisionsHandler(d.DB), DivisionsPrivLevel, Authenticated, nil},
		//HwInfo
		{1.2, http.MethodGet, `hwinfo-wip/?(\.json)?$`, hwInfoHandler(d.DB), HWInfoPrivLevel, Authenticated, nil},
		//Parameters
		{1.3, http.MethodGet, `parameters/?(\.json)?$`, parametersHandler(d.DB), ParametersPrivLevel, Authenticated, nil},
		//Regions
		{1.2, http.MethodGet, `regions/?(\.json)?$`, regionsHandler(d.DB), RegionsPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `regions/{id}$`, regionsHandler(d.DB), RegionsPrivLevel, Authenticated, nil},
		//Servers
		// explicitly passed to legacy system until fully implemented.  Auth handled by legacy system.
		{1.2, http.MethodGet, `servers/checks$`, handlerToFunc(proxyHandler), 0, NoAuth, []Middleware{}},
		{1.2, http.MethodGet, `servers/details$`, handlerToFunc(proxyHandler), 0, NoAuth, []Middleware{}},
		{1.2, http.MethodGet, `servers/status$`, handlerToFunc(proxyHandler), 0, NoAuth, []Middleware{}},
		{1.2, http.MethodGet, `servers/totals$`, handlerToFunc(proxyHandler), 0, NoAuth, []Middleware{}},

		{1.2, http.MethodGet, `servers/?(\.json)?$`, serversHandler(d.DB), ServersPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `servers/{id}$`, serversHandler(d.DB), ServersPrivLevel, Authenticated, nil},
		{1.2, http.MethodPost, `servers/{id}/deliveryservices$`, assignDeliveryServicesToServerHandler(d.DB), auth.PrivLevelOperations, Authenticated, nil},
		{1.2, http.MethodGet, `servers/{host_name}/update_status$`, getServerUpdateStatusHandler(d.DB), auth.PrivLevelReadOnly, Authenticated, nil},

		//SSLKeys deliveryservice endpoints here that are marked  marked as '-wip' need to have tenancy checks added
		{1.2, http.MethodGet, `deliveryservices-wip/xmlId/{xmlID}/sslkeys$`, getDeliveryServiceSSLKeysByXmlIDHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		{1.2, http.MethodGet, `deliveryservices-wip/hostname/{hostName}/sslkeys$`, getDeliveryServiceSSLKeysByHostNameHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		{1.2, http.MethodPut, `deliveryservices-wip/hostname/{hostName}/sslkeys$`, addDeliveryServiceSSLKeysHandler(d.DB, d.Config), auth.PrivLevelAdmin, Authenticated, nil},
		//Statuses
		{1.2, http.MethodGet, `statuses/?(\.json)?$`, statusesHandler(d.DB), StatusesPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `statuses/{id}$`, statusesHandler(d.DB), StatusesPrivLevel, Authenticated, nil},
		//System
		{1.2, http.MethodGet, `system/info/?(\.json)?$`, systemInfoHandler(d.DB), SystemInfoPrivLevel, Authenticated, nil},

		//Phys_Locations
		{1.2, http.MethodGet, `phys_locations/?(\.json)?$`, phys_location.PhysLocationsHandler(d.DB), phys_location.PhysLocationsPrivLevel, Authenticated, nil},
		{1.2, http.MethodGet, `phys_locations/{id}$`, phys_location.PhysLocationsHandler(d.DB), phys_location.PhysLocationsPrivLevel, Authenticated, nil},
	}
	return routes, proxyHandler, nil
}

// RootHandler returns the / handler for the service, which reverse-proxies the old Perl Traffic Ops
func rootHandler(d ServerData) http.Handler {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(d.Config.ProxyTimeout) * time.Second,
			KeepAlive: time.Duration(d.Config.ProxyKeepAlive) * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   time.Duration(d.Config.ProxyTLSTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(d.Config.ProxyReadHeaderTimeout) * time.Second,
		//Other knobs we can turn: ExpectContinueTimeout,IdleConnTimeout
	}
	rp := httputil.NewSingleHostReverseProxy(d.URL)
	rp.Transport = tr

	var errorLogger interface{}
	errorLogger, err := tclog.GetLogWriter(d.Config.ErrorLog())
	if err != nil {
		tclog.Errorln("could not create error log writer for proxy: ", err)
	}
	if errorLogger != nil {
		rp.ErrorLog = log.New(errorLogger.(io.Writer), "proxy error: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC) //if we don't provide a logger to the reverse proxy it logs to stdout/err and is lost when ran by a script.
		riak.SetErrorLogger(log.New(errorLogger.(io.Writer), "riak error: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC))
	}
	var infoLogger interface{}
	infoLogger, err = tclog.GetLogWriter(d.Config.InfoLog())
	if err != nil {
		tclog.Errorln("could not create info log writer for proxy: ", err)
	}
	if infoLogger != nil {
		riak.SetLogger(log.New(infoLogger.(io.Writer), "riak info: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC))
	}
	tclog.Debugf("our reverseProxy: %++v\n", rp)
	tclog.Debugf("our reverseProxy's transport: %++v\n", tr)
	loggingProxyHandler := wrapAccessLog(d.Secrets[0], rp)

	managerHandler := CreateThrottledHandler(loggingProxyHandler, d.BackendMaxConnections["mojolicious"])
	return managerHandler
}

//CreateThrottledHandler takes a handler, and a max and uses a channel to insure the handler is used concurrently by only max number of routines
func CreateThrottledHandler(handler http.Handler, maxConcurrentCalls int) ThrottledHandler {
	return ThrottledHandler{handler, make(chan struct{}, maxConcurrentCalls)}
}

type ThrottledHandler struct {
	Handler http.Handler
	ReqChan chan struct{}
}

func (m ThrottledHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.ReqChan <- struct{}{}
	defer func() { <-m.ReqChan }()
	m.Handler.ServeHTTP(w, r)
}
