package db

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

//Common contains all the necessary muck to work with postgres
type Common struct {
	DB *sqlx.DB
}

type PreProcessFuncs struct {
	//ConvertEntityFn can be leveraged to ensure the appropriate conversion of the interface
	ConvertEntityFn func(interface{}) (interface{}, error)
	//ValidateEntityFn is used to check whether the payload for the store is valid
	ValidateEntityFn func(interface{}) (interface{}, error)
	//GetErrorMessageFn generates the error message for the entity
	GetErrorMessageFn func(interface{}, string) error
	//CheckForEmptinessFn checks whether the payload is empty
	CheckForEmptinessFn func(interface{}) error
}

//Store will store the entity to the underlying storage
//If the storage fails for any reason the error will be surfaced back to the caller
func (c *Common) Store(tableName string, entity interface{}, pProc PreProcessFuncs) (err error) {
	converted, err := pProc.ConvertEntityFn(entity)
	if err != nil {
		return
	}
	validated, err := pProc.ValidateEntityFn(converted)
	if err != nil {
		return
	}
	insertStatement := ToInsertStatement(tableName, validated)
	_, err = c.DB.NamedExec(insertStatement, converted)
	if sqlErr, ok := err.(*pq.Error); ok {
		log.Printf("DBCommon:Store:Error:%v", sqlErr)
		err = pProc.GetErrorMessageFn(converted, sqlErr.Code.Class().Name())
	}
	return err
}

//Upsert is ...
func (c *Common) Upsert(tableName string, entity interface{}, pProc PreProcessFuncs) (err error) {
	converted, err := pProc.ConvertEntityFn(entity)
	if err != nil {
		return
	}
	validated, err := pProc.ValidateEntityFn(converted)
	if err != nil {
		return
	}
	upsertStatement, upsertArgs, err := ToUpsertStatement(tableName, validated)
	if err != nil {
		return
	}
	fmt.Printf("Upsert statement %v\n", upsertStatement)
	_, err = c.DB.NamedExec(upsertStatement, upsertArgs)
	fmt.Printf("Found error: %v\n", err)
	if sqlErr, ok := err.(*pq.Error); ok {
		err = pProc.GetErrorMessageFn(converted, sqlErr.Code.Class().Name())
	}
	return err
}

//Get will retrieve the entity from the underlying storage
//It will populate the results slice matching the query. If the Get fails for
//any reason, the error will be surfaced back to the caller
func (c *Common) Get(tableName string, query interface{}, results interface{}, pProc PreProcessFuncs) (err error) {
	convertedQuery, err := pProc.ConvertEntityFn(query)
	if err != nil {
		return
	}
	err = pProc.CheckForEmptinessFn(query)
	if err != nil {
		return
	}
	validatedQuery, err := pProc.ValidateEntityFn(convertedQuery)
	if err != nil {
		return
	}
	//TODO: Check if the convertedQuery is empty
	queryString, queryArgs := ToSelectStatement(tableName, validatedQuery)
	err = c.DB.Select(results, queryString, queryArgs...)
	if err != nil {
		log.Printf("DBCommon:Store:Error:%v", err)
	}
	return
}

//Update will update the entity in the underlying storage
//The update will leverage the underlying versionID to ensure
//optimistic concurrency control.
//If there's a versionID mismatch the update will throw a
//VersionMismatchError.
func (c *Common) Update(tableName string, entity interface{}, pProc PreProcessFuncs) (versionID *int64, err error) {
	converted, err := pProc.ConvertEntityFn(entity)
	if err != nil {
		fmt.Printf("DBCommon: Failed to convert:%v\n", err)
		return nil, err
	}
	updateStatement, updateArgs, newVersionID, err := ToUpdateStatement(tableName, converted)

	if err != nil {
		fmt.Printf("Issue updating DB due to %v\n", err)
		return nil, err
	}
	result, err := c.DB.NamedExec(updateStatement, updateArgs)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("%v", "Update failed, most likely due to stale version")
	}
	versionID = &newVersionID
	return
}

//CreateTable will create a table with the provided schema
func (c *Common) CreateTable(schema string) {
	//TODO: Super Dangerous! Refactor to generate actual CREATE statement from the payload and then execute
	c.DB.MustExec(schema)
}

//TruncateTable will empty the table
func (c *Common) TruncateTable(tableName string) {
	c.DB.MustExec(fmt.Sprintf(`TRUNCATE %v`, tableName))
}

//ToInsertStatement ...
func ToInsertStatement(tableName string, insertPayload interface{}) string {
	attributes := getFieldBreakdown("db", insertPayload)
	insertColumns := generateClause(attributes, ",", func(k string) string {
		return k
	})
	insertValues := generateClause(attributes, ",", func(k string) string {
		return fmt.Sprintf(":%v", k)
	})
	return fmt.Sprintf("INSERT INTO %v (%v) VALUES (%v)", tableName, insertColumns, insertValues)
}

//ToUpsertStatement ...
func ToUpsertStatement(tableName string, upsertPayload interface{}) (sqlStatement string, sqlArgs map[string]interface{}, err error) {
	insertStatement := ToInsertStatement(tableName, upsertPayload)
	conflictAttributes := getFieldBreakdown("conflict", upsertPayload)
	upsertAttributes := getFieldBreakdown("upsert", upsertPayload)
	upsertKeyAttributes := getFieldBreakdown("upsertkey", upsertPayload)
	sqlArgs = getFieldBreakdown("db", upsertPayload)
	//Update the versionID
	oldVersionID := sqlArgs["versionid"]
	vID, ok := oldVersionID.(int64)
	if !ok {
		err = fmt.Errorf("please supply versionID before performing any DB operations")
		return
	}
	newVersionID := vID + 1 //Increment the versionID
	//Remove the versionID key from the updatable attributes
	//delete(sqlArgs, "versionid")

	conflictClause := generateClause(conflictAttributes, ",", func(k string) string {
		return k
	})
	updateClause := generateSetClause(upsertAttributes)
	updatePredicateClause := generateTablePredicateClause(tableName, upsertKeyAttributes)
	sqlStatement = fmt.Sprintf("%v ON CONFLICT (%v) DO UPDATE SET %v,versionid=%v WHERE %v", insertStatement, conflictClause, updateClause, newVersionID, updatePredicateClause)
	fmt.Printf("Update statement is %v\n", sqlStatement)
	return
}

//ToSelectStatement returns the select predicate and the
//args that will be fed into the execute
func ToSelectStatement(tableName string, selectPayload interface{}) (string, []interface{}) {
	queryFieldValue := getFieldBreakdown("db", selectPayload)
	queryFieldValues := []interface{}{}
	var queryPredicate strings.Builder
	queryPredicate.WriteString(fmt.Sprintf("SELECT * FROM %v WHERE ", tableName))

	var i int
	//Sort the queryFieldValue before aggregating
	sortedQueryFieldKeys := make([]string, len(queryFieldValue))
	for k := range queryFieldValue {
		sortedQueryFieldKeys[i] = k
		i++
	}

	sort.Strings(sortedQueryFieldKeys)

	for idx, k := range sortedQueryFieldKeys {
		if idx != 0 {
			queryPredicate.WriteString(" AND ")
		}
		queryFieldValues = append(queryFieldValues, queryFieldValue[k])
		queryPredicate.WriteString(fmt.Sprintf("%v=$%v", k, idx+1))
		i++
	}
	return queryPredicate.String(), queryFieldValues
}

//ToUpdateStatement is ...
func ToUpdateStatement(tableName string, updatePayload interface{}) (sqlStatement string, sqlArgs map[string]interface{}, newVersionID int64, err error) {
	//Generate map of predicate from the TaskRun
	//TODO: Validate that the update and the key attributes are populated
	//fmt.Printf("Creating update statement for payload:%v\n", updatePayload)
	updatableAttributes := getFieldBreakdown("update", updatePayload)
	keyAttributes := getFieldBreakdown("key", updatePayload)
	sqlArgs = getFieldBreakdown("db", updatePayload)

	//Update the versionID
	oldVersionID := sqlArgs["versionid"]
	vID, ok := oldVersionID.(int64)
	if !ok {
		err = fmt.Errorf("please supply versionID before performing any DB operations")
		return
	}
	newVersionID = vID + 1 //Increment the versionID
	//Remove the versionID key from the updatable attributes
	delete(updatableAttributes, "versionid")

	//Generate SET from updatableAttributes
	setClause := generateSetClause(updatableAttributes)
	predicateClause := generatePredicateClause(keyAttributes)
	sqlStatement = fmt.Sprintf("UPDATE %v SET %v,versionid=%v WHERE %v", tableName, setClause, newVersionID, predicateClause)
	//fmt.Printf("Generated update statement:%v\n", sqlStatement)
	return
}

func generateSetClause(attributes map[string]interface{}) string {
	patternFunc := func(k string) string {
		return fmt.Sprintf("%v=:%v", k, k)
	}
	return generateClause(attributes, ",", patternFunc)
}

func generatePredicateClause(attributes map[string]interface{}) string {
	patternFunc := func(k string) string {
		return fmt.Sprintf("%v=:%v", k, k)
	}
	return generateClause(attributes, " AND ", patternFunc)
}

func generateTablePredicateClause(tableName string, attributes map[string]interface{}) string {
	patternFunc := func(k string) string {
		return fmt.Sprintf("%v.%v=:%v", tableName, k, k)
	}
	return generateClause(attributes, " AND ", patternFunc)
}

func generateClause(attributes map[string]interface{},
	delimiter string, patternFunc func(key string) string) string {

	var i int
	//Sort the queryFieldValue before aggregating
	sortedQueryFieldKeys := make([]string, len(attributes))
	for k := range attributes {
		sortedQueryFieldKeys[i] = k
		i++
	}

	sort.Strings(sortedQueryFieldKeys)

	var clauseBuilder strings.Builder
	for idx, key := range sortedQueryFieldKeys {
		if idx != 0 {
			clauseBuilder.WriteString(delimiter)
		}
		clauseBuilder.WriteString(patternFunc(key))
	}
	return clauseBuilder.String()
}

func getFieldBreakdown(tag string, payload interface{}) (queryFieldValue map[string]interface{}) {
	queryFieldValue = make(map[string]interface{})
	v := reflect.ValueOf(payload)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	typ := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fi := typ.Field(i)
		tag := fi.Tag.Get(tag)
		if len(strings.TrimSpace(tag)) > 0 {
			cType := v.Field(i).Type()
			fieldValue := v.Field(i).Interface()
			//Check if the value is zero
			if cType.Kind() == reflect.Bool || !reflect.DeepEqual(reflect.Zero(cType).Interface(), fieldValue) {
				queryFieldValue[tag] = fieldValue
			}
		}
	}
	return
}
