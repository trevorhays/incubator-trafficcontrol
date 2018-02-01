package cdn

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
	"net/url"
	"testing"
	"time"

	"github.com/apache/incubator-trafficcontrol/lib/go-tc"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/auth"
	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/test"
	"github.com/jmoiron/sqlx"

	sqlmock "gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func getTestCDNs() []tc.CDN {
	cdns := []tc.CDN{}
	testCDN := tc.CDN{
		DNSSECEnabled: false,
		DomainName:    "domainName",
		ID:            1,
		Name:          "cdn1",
		LastUpdated:   tc.Time{Time: time.Now()},
	}
	cdns = append(cdns, testCDN)

	testCDN2 := testCDN
	testCDN2.Name = "cdn2"
	testCDN2.DomainName = "domain.net"
	cdns = append(cdns, testCDN2)

	return cdns
}

func TestReadCDNs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")
	defer db.Close()

	refType := GetRefType()

	testCDNs := getTestCDNs()
	cols := test.ColsFromStructByTag("db", tc.CDN{})
	rows := sqlmock.NewRows(cols)

	//TODO: drichardson - build helper to add these Rows from the struct values
	//                    or by CSV if types get in the way
	for _, ts := range testCDNs {
		rows = rows.AddRow(
			ts.DNSSECEnabled,
			ts.DomainName,
			ts.ID,
			ts.LastUpdated,
			ts.Name,
		)
	}
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	v := url.Values{}
	v.Set("dsId", "1")

	servers, err, _ := refType.Read(db, v, auth.CurrentUser{})
	if err != nil {
		t.Errorf("cdn.Read expected: nil error, actual: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("cdn.Read expected: len(servers) == 2, actual: %v", len(servers))
	}
}
