import _lib
import _transfers
import _sql
import _blocks
import re
import time
import startnode
import transactions

datadir = ""

def aftertest(testfilter):
    global datadir
    
    if datadir != "":
        startnode.StopNode(datadir)
        
def test(testfilter):
    global datadir
    
    _lib.StartTestGroup("SQL basic")

    _lib.CleanTestFolders()
    datadir = _lib.CreateTestFolder()

    startnode.StartNodeWithoutBlockchain(datadir)
    address = startnode.InitBockchain(datadir)
    startnode.StartNode(datadir, address, '30000')
    
    _lib.StartTestGroup("Do transactions")

    transactions.GetUnapprovedTransactionsEmpty(datadir)
    
    tx1 = _sql.ExecuteSQL(datadir,address,"CREATE TABLE test (a INT auto_increment PRIMARY KEY, b VARCHAR(20))")
    
    # check new table exists
    tables = _lib.DBGetRows(datadir,"SHOW TABLES")
    found = False
    for table in tables:
        if table[0] == "test":
            found = True
            break
    
    _lib.FatalAssert(found, "Table not found in the DB")
    
    blocks = _blocks.WaitBlocks(datadir, 2)
    
    tx2 = _sql.ExecuteSQL(datadir,address,"INSERT INTO test SET b='row1'")
    tx3 = _sql.ExecuteSQL(datadir,address,"INSERT INTO test SET a=2,b='row2'")
    time.sleep(1)
    tx4 = _sql.ExecuteSQL(datadir,address,"INSERT INTO test (b) VALUES ('row3')")
    
    rows = _lib.DBGetRows(datadir,"SELECT * FROM test")
    
    _lib.FatalAssert(len(rows) == 3, "Must be 3 rows in a table")
    
    blocks = _blocks.WaitBlocks(datadir, 3)
    
    time.sleep(1)# while all caches are cleaned
    
    txlist = transactions.GetUnapprovedTransactions(datadir)
    _lib.FatalAssert(len(txlist) == 1,"Should be 1 unapproved transaction")
    
    startnode.StopNode(datadir)
    datadir = ""
    
    #_lib.RemoveTestFolder(datadir)
    _lib.EndTestGroupSuccess()
    

    