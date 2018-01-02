package phys_location

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
	"context"
	"errors"
	"fmt"

	"github.com/apache/incubator-trafficcontrol/lib/go-log"
	"github.com/apache/incubator-trafficcontrol/lib/go-tc"

	"github.com/apache/incubator-trafficcontrol/traffic_ops/traffic_ops_golang/dbhelpers"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

//we need a type alias to define functions on
type TOPHYSLOCATION tc.PhysLocation

//the refType is passed into the handlers where a copy of its type is used to decode the json.
var refType = TOPHYSLOCATION(tc.PhysLocation{})

func GetRefType() *TOPHYSLOCATION {
	return &refType
}

//Implementation of the Identifier, Validator interface functions
func (phys_location *TOPHYSLOCATION) GetID() int {
	return phys_location.ID
}

func (phys_location *TOPHYSLOCATION) GetAuditName() string {
	return phys_location.Name
}

func (phys_location *TOPHYSLOCATION) GetType() string {
	return "phys_location"
}

func (phys_location *TOPHYSLOCATION) SetID(i int) {
	phys_location.ID = i
}

func (phys_location *TOPHYSLOCATION) Validate() []error {
	errs := []error{}
	if len(phys_location.Name) < 1 {
		errs = append(errs, errors.New(`Physical Location 'name' is required.`))
	}

	return errs
}

//The TOPHYSLOCATION implementation of the Updater interface
//all implementations of Updater should use transactions and return the proper errorType
//ParsePQUniqueConstraintError is used to determine if a phys_location with conflicting values exists
//if so, it will return an errorType of DataConflict and the type should be appended to the
//generic error message returned
func (phys_location *TOPHYSLOCATION) Update(db *sqlx.DB, ctx context.Context) (error, tc.ApiErrorType) {
	tx, err := db.Beginx()
	defer func() {
		if tx == nil {
			return
		}
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	if err != nil {
		log.Error.Printf("could not begin transaction: %v", err)
		return tc.DBError, tc.SystemError
	}
	log.Debugf("about to run exec query: %s with phys_location: %++v", updatePhysLocationQuery(), phys_location)
	resultRows, err := tx.NamedQuery(updatePhysLocationQuery(), phys_location)
	if err != nil {
		if err, ok := err.(*pq.Error); ok {
			err, eType := dbhelpers.ParsePQUniqueConstraintError(err)
			if eType == tc.DataConflictError {
				return errors.New("a phys_location with " + err.Error()), eType
			}
			return err, eType
		} else {
			log.Errorf("received error: %++v from update execution", err)
			log.Errorf("%+v\n",phys_location)
			return tc.DBError, tc.SystemError
		}
	}
	var lastUpdated tc.Time
	rowsAffected := 0
	for resultRows.Next() {
		rowsAffected++
		if err := resultRows.Scan(&lastUpdated); err != nil {
			log.Error.Printf("could not scan lastUpdated from insert: %s\n", err)
			return tc.DBError, tc.SystemError
		}
	}
	log.Debugf("lastUpdated: %++v", lastUpdated)
	phys_location.LastUpdated = lastUpdated
	if rowsAffected != 1 {
		if rowsAffected < 1 {
			return errors.New("no phys_location found with this id"), tc.DataMissingError
		} else {
			return fmt.Errorf("this update affected too many rows: %d", rowsAffected), tc.SystemError
		}
	}
	return nil, tc.NoError
}

//The TOPHYSLOCATION implementation of the Inserter interface
//all implementations of Inserter should use transactions and return the proper errorType
//ParsePQUniqueConstraintError is used to determine if a phys_location with conflicting values exists
//if so, it will return an errorType of DataConflict and the type should be appended to the
//generic error message returned
//The insert sql returns the id and lastUpdated values of the newly inserted phys_location and have
//to be added to the struct
func (phys_location *TOPHYSLOCATION) Insert(db *sqlx.DB, ctx context.Context) (error, tc.ApiErrorType) {
	tx, err := db.Beginx()
	defer func() {
		if tx == nil {
			return
		}
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	if err != nil {
		log.Error.Printf("could not begin transaction: %v", err)
		return tc.DBError, tc.SystemError
	}
	resultRows, err := tx.NamedQuery(insertPhysLocationQuery(), phys_location)
	if err != nil {
		if err, ok := err.(*pq.Error); ok {
			err, eType := dbhelpers.ParsePQUniqueConstraintError(err)
			return errors.New("a phys_location with " + err.Error()), eType
		} else {
			log.Errorf("received non pq error: %++v from create execution", err)
			return tc.DBError, tc.SystemError
		}
	}
	var id int
	var lastUpdated tc.Time
	rowsAffected := 0
	for resultRows.Next() {
		rowsAffected++
		if err := resultRows.Scan(&id, &lastUpdated); err != nil {
			log.Error.Printf("could not scan id from insert: %s\n", err)
			return tc.DBError, tc.SystemError
		}
	}
	if rowsAffected == 0 {
		err = errors.New("no phys_location was inserted, no id was returned")
		log.Errorln(err)
		return tc.DBError, tc.SystemError
	} else if rowsAffected > 1 {
		err = errors.New("too many ids returned from phys_location insert")
		log.Errorln(err)
		return tc.DBError, tc.SystemError
	}
	phys_location.SetID(id)
	phys_location.LastUpdated = lastUpdated
	return nil, tc.NoError
}

//The TOPHYSLOCATION implementation of the Deleter interface
//all implementations of Deleter should use transactions and return the proper errorType
func (phys_location *TOPHYSLOCATION) Delete(db *sqlx.DB, ctx context.Context) (error, tc.ApiErrorType) {
	tx, err := db.Beginx()
	defer func() {
		if tx == nil {
			return
		}
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	if err != nil {
		log.Error.Printf("could not begin transaction: %v", err)
		return tc.DBError, tc.SystemError
	}
	log.Debugf("about to run exec query: %s with phys_location: %++v", deletePhysLocationQuery(), phys_location)
	result, err := tx.NamedExec(deletePhysLocationQuery(), phys_location)
	if err != nil {
		log.Errorf("received error: %++v from delete execution", err)
		return tc.DBError, tc.SystemError
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return tc.DBError, tc.SystemError
	}
	if rowsAffected != 1 {
		if rowsAffected < 1 {
			return errors.New("no phys_location with that id found"), tc.DataMissingError
		} else {
			return fmt.Errorf("this create affected too many rows: %d", rowsAffected), tc.SystemError
		}
	}
	return nil, tc.NoError
}

func updatePhysLocationQuery() string {
	query := `UPDATE
phys_location SET
address=:address,
city=:city,
comments=:comments,
email=:email,
name=:name,
phone=:phone,
poc=:poc,
region=:regionId,
short_name=:short_name,
state=:state,
zip=:zip
WHERE id=:id RETURNING last_updated`
	return query
}

func insertPhysLocationQuery() string {
	query := `INSERT INTO phys_location (
address,
city,
comments,
email,
name,
phone,
poc,
region,
short_name,
state,
zip) VALUES (
:address,
:city,
:comments,
:email,
:name,
:phone,
:poc,
:regionId,
:short_name,
:state,
:zip) RETURNING id,last_updated`
	return query
}

func deletePhysLocationQuery() string {
	query := `DELETE FROM phys_location
WHERE id=:id`
	return query
}

