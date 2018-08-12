package database

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"database/sql"

	"github.com/JamesStewy/go-mysqldump"
	"github.com/gelembjuk/oursql/lib/utils"
	_ "github.com/go-sql-driver/mysql"
)

const (
	ClassNameNodes                  = "nodes"
	ClassNameBlockchain             = "blockchain"
	ClassNameTransactions           = "transactions"
	ClassNameUnapprovedTransactions = "unapprovedtransactions"
	ClassNameUnspentOutputs         = "unspentoutputs"
)

type MySQLDBManager struct {
	Logger     *utils.LoggerMan
	Config     DatabaseConfig
	conn       *sql.DB
	openedConn bool
	SessID     string
}

func (bdm *MySQLDBManager) QM() DBQueryManager {
	return bdm
}

func (bdm *MySQLDBManager) SetConfig(config DatabaseConfig) error {
	bdm.Config = config

	return nil
}
func (bdm *MySQLDBManager) SetLogger(logger *utils.LoggerMan) error {
	bdm.Logger = logger

	return nil
}

// try to set up a connection to DB. and close it then
func (bdm *MySQLDBManager) CheckConnection() error {
	conn, err := bdm.getConnection()

	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Query("SHOW TABLES")

	if err != nil {
		return err
	}

	return nil
}

// set status of connection to open
func (bdm *MySQLDBManager) OpenConnection() error {
	//bdm.Logger.Trace.Println("open connection for " + reason)
	if bdm.openedConn {
		return nil
	}
	// real connection will be done when first object is created
	bdm.openedConn = true

	bdm.conn = nil

	return nil
}
func (bdm *MySQLDBManager) CloseConnection() error {
	if !bdm.openedConn {
		return nil
	}

	if bdm.conn != nil {
		bdm.conn.Close()
		bdm.conn = nil
	}

	bdm.openedConn = false
	return nil
}

func (bdm *MySQLDBManager) IsConnectionOpen() bool {
	return bdm.openedConn
}

// create empty database. must create all
// creates tables for BC
func (bdm *MySQLDBManager) InitDatabase() error {

	bdm.OpenConnection()

	defer bdm.CloseConnection()

	bc, err := bdm.GetBlockchainObject()

	if err != nil {
		return err
	}

	err = bc.InitDB()

	if err != nil {
		return err
	}

	txs, err := bdm.GetTransactionsObject()

	if err != nil {
		return err
	}

	err = txs.InitDB()

	if err != nil {
		return err
	}

	utx, err := bdm.GetUnapprovedTransactionsObject()

	if err != nil {
		return err
	}

	err = utx.InitDB()

	if err != nil {
		return err
	}

	uos, err := bdm.GetUnspentOutputsObject()

	if err != nil {
		return err
	}

	err = uos.InitDB()

	if err != nil {
		return err
	}

	ns, err := bdm.GetNodesObject()

	if err != nil {
		return err
	}

	err = ns.InitDB()

	if err != nil {
		return err
	}

	return nil
}

// Check if database was already inited
func (bdm *MySQLDBManager) CheckDBExists() (bool, error) {
	bc, err := bdm.GetBlockchainObject()

	if err != nil {
		return false, nil
	}

	tophash, err := bc.GetTopHash()

	if err != nil {
		return false, nil
	}

	if len(tophash) > 0 {
		return true, nil
	}

	return false, nil
}

// returns BlockChain Database structure. does all init
func (bdm *MySQLDBManager) GetBlockchainObject() (BlockchainInterface, error) {
	conn, err := bdm.getConnection()

	if err != nil {
		return nil, err
	}

	bc := Blockchain{}
	bc.DB = &MySQLDB{conn, bdm.Config.TablesPrefix, bdm.Logger}

	return &bc, nil
}

// returns Transaction Index Database structure. does al init
func (bdm *MySQLDBManager) GetTransactionsObject() (TranactionsInterface, error) {
	conn, err := bdm.getConnection()

	if err != nil {
		return nil, err
	}

	txs := Tranactions{}
	txs.DB = &MySQLDB{conn, bdm.Config.TablesPrefix, bdm.Logger}

	return &txs, nil
}

// returns Unapproved Transaction Database structure. does al init
func (bdm *MySQLDBManager) GetUnapprovedTransactionsObject() (UnapprovedTransactionsInterface, error) {
	conn, err := bdm.getConnection()

	if err != nil {
		return nil, err
	}

	uos := UnapprovedTransactions{}
	uos.DB = &MySQLDB{conn, bdm.Config.TablesPrefix, bdm.Logger}

	return &uos, nil
}

// returns Unspent Transactions Database structure. does al init
func (bdm *MySQLDBManager) GetUnspentOutputsObject() (UnspentOutputsInterface, error) {
	conn, err := bdm.getConnection()

	if err != nil {
		return nil, err
	}

	uts := UnspentOutputs{}
	uts.DB = &MySQLDB{conn, bdm.Config.TablesPrefix, bdm.Logger}

	return &uts, nil
}

// returns Nodes Database structure. does al init
func (bdm *MySQLDBManager) GetNodesObject() (NodesInterface, error) {
	conn, err := bdm.getConnection()

	if err != nil {
		return nil, err
	}

	ns := Nodes{}
	ns.DB = &MySQLDB{conn, bdm.Config.TablesPrefix, bdm.Logger}

	return &ns, nil
}

// returns DB connection, creates it if needed .
func (bdm *MySQLDBManager) getConnection() (*sql.DB, error) {

	if !bdm.openedConn {
		return nil, errors.New("Connection was not inited")
	}

	if bdm.conn != nil {
		return bdm.conn, nil
	}

	db, err := sql.Open("mysql", bdm.Config.GetMySQLConnString())

	if err != nil {
		return nil, err
	}
	//db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	bdm.conn = db

	return db, nil
}

func (bdm *MySQLDBManager) GetLockerObject() DatabaseLocker {
	return nil
}
func (bdm *MySQLDBManager) SetLockerObject(lockerobj DatabaseLocker) {

}

func (bdm *MySQLDBManager) Dump(file string) error {
	conn, err := bdm.getConnection()

	if err != nil {
		return err
	}
	// Register database with mysqldump
	dumpDir, _ := filepath.Abs(filepath.Dir(file))
	dumpFilename := filepath.Base(file)

	if strings.HasSuffix(dumpFilename, ".sql") {
		dumpFilename = dumpFilename[:len(dumpFilename)-4]
	}
	fmt.Printf("file name %s", dumpFilename)
	dumper, err := mysqldump.Register(conn, dumpDir, dumpFilename)

	if err != nil {
		return err
	}

	// Dump database to file
	_, err = dumper.Dump()

	if err != nil {
		return err
	}

	dumper.Close()
	return nil
}
func (bdm *MySQLDBManager) Restore(file string) error {
	connstr := bdm.Config.GetMySQLConnString() + "?multiStatements=true"
	db, err := sql.Open("mysql", connstr)

	if err != nil {
		return err
	}

	// load file to string
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	sql := string(b)

	_, err = db.Exec(sql)

	return err
}

// execute query.
func (bdm MySQLDBManager) ExecuteSQL(sql string) error {
	db, err := bdm.getConnection()

	if err != nil {
		return err
	}
	_, err = db.Exec(sql)
	return err
}

// execute EXPLAIN query to check if sql query is correct and what is type and which table it affects
func (bdm MySQLDBManager) ExecuteSQLExplain(sql string) (r SQLExplainInfo, err error) {
	explainRes, err := bdm.ExecuteSQLSelectRow("EXPLAIN " + sql)

	if err != nil {
		return
	}
	r.Id = explainRes["Id"]
	r.SelectType = explainRes["SelectType"]
	r.Table = explainRes["Table"]
	r.Partitions = explainRes["Partitions"]
	r.Type = explainRes["Type"]
	r.PossibleKeys = explainRes["PossibleKeys"]
	r.Key = explainRes["Key"]
	r.KeyLen, _ = strconv.Atoi(explainRes["KeyLen"])
	r.Ref = explainRes["Ref"]
	r.Rows, _ = strconv.Atoi(explainRes["Rows"])
	r.Filtered = explainRes["Filtered"]
	r.Extra = explainRes["Extra"]

	return r, err
}

// get primary key column name for a table
func (bdm MySQLDBManager) ExecuteSQLPrimaryKey(table string) (column string, err error) {
	row, err := bdm.ExecuteSQLSelectRow("SHOW KEYS FROM " + table + " WHERE Key_name = 'PRIMARY'")

	if err != nil {
		return
	}
	column = row["Column_name"]
	return
}

// get row by table name and primary key value
func (bdm MySQLDBManager) ExecuteSQLRowByKey(table string, priKeyVal string) (data map[string]string, err error) {
	return
}

// get single row as a map
func (bdm MySQLDBManager) ExecuteSQLSelectRow(sqlcommand string) (data map[string]string, err error) {
	db, err := bdm.getConnection()

	if err != nil {
		return
	}

	rows, err := db.Query(sqlcommand)

	if err != nil {
		return
	}

	cols, err := rows.Columns()

	if err != nil {
		return
	}

	data = make(map[string]string)

	if rows.Next() {
		columns := make([]sql.NullString, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i, _ := range columns {
			columnPointers[i] = &columns[i]
		}

		err = rows.Scan(columnPointers...)

		if err != nil {
			return
		}
		for i, colName := range cols {
			val := ""

			if columns[i].Valid {
				val = columns[i].String
			}

			data[colName] = val
		}
	}

	return
}

func (bdm MySQLDBManager) ExecuteSQLNextKeyValue(table string) (string, error) {
	row, err := bdm.ExecuteSQLSelectRow("SHOW TABLE STATUS LIKE '" + table + "'")

	if err != nil {
		return "", err
	}
	return row["Auto_increment"], nil
}
