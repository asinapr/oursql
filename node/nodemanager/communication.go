package nodemanager

import (
	"github.com/gelembjuk/oursql/lib"
	"github.com/gelembjuk/oursql/lib/net"
	"github.com/gelembjuk/oursql/lib/utils"

	"github.com/gelembjuk/oursql/node/structures"
)

type communicationManager struct {
	logger *utils.LoggerMan
	node   *Node
}

// Send own version to all known nodes

func (n communicationManager) sendVersionToNodes(nodes []net.NodeAddr, bestHeight int) {

	if len(nodes) == 0 {
		nodes = n.node.NodeNet.Nodes
	}

	for _, node := range nodes {
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}
		n.node.NodeClient.SendVersion(node, bestHeight)
	}
}

// Send transaction to all known nodes. This wil send only hash and node hash to check if hash exists or no
func (n *communicationManager) sendTransactionToAll(tx *structures.Transaction) error {
	n.logger.Trace.Printf("Send transaction to %d nodes", len(n.node.NodeNet.Nodes))

	// decide how to send, async or sync
	if n.node.NodeNet.CheckHadInputConnects() {
		// can send async. Other nodes can connect to us
		return n.sendTransactionToAllASync(tx)
	}
	// use sync mode.
	return n.sendTransactionToAllSync(tx)
}

// Send tranaction ID to all nodes in async mode. We expect nodes will call us back to get TX
func (n *communicationManager) sendTransactionToAllASync(tx *structures.Transaction) error {
	n.logger.Trace.Printf("Send transaction to %d nodes in async mode", len(n.node.NodeNet.Nodes))

	for i, node := range n.node.NodeNet.Nodes {
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}
		n.logger.Trace.Printf("Send TX %x to %s", tx.GetID(), node.NodeAddrToString())
		err := n.node.NodeClient.SendInv(node, "tx", [][]byte{tx.GetID()})
		n.node.NodeNet.HookNeworkOperationResult(err, i) // to know if this node is available
	}
	return nil
}

// Send tranaction ID to all nodes in sync mode. We just send full TX to all known nodes and they decide what to do
func (n *communicationManager) sendTransactionToAllSync(tx *structures.Transaction) error {
	n.logger.Trace.Printf("Send transaction to %d nodes in sync mode", len(n.node.NodeNet.Nodes))

	// serialize TX
	txser, err := structures.SerializeTransaction(tx)

	if err != nil {
		n.logger.Error.Printf("Error on TX ser %s", err.Error())
		return err
	}

	for i, node := range n.node.NodeNet.Nodes {
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}
		n.logger.Trace.Printf("Send TX %x to %s", tx.GetID(), node.NodeAddrToString())
		err := n.node.NodeClient.SendTx(node, txser)
		n.node.NodeNet.HookNeworkOperationResult(err, i) // to know if this node is available
	}
	return nil
}

// Send block to all known nodes
// This is used in case when new block was received from other node or
// created by this node. We will notify our network about new block
// But not send full block, only hash and previous hash. So, other can copy it
// Address from where we get it will be skipped
func (n *communicationManager) SendBlockToAll(newBlock *structures.Block, skipaddr net.NodeAddr) error {
	n.logger.Trace.Printf("Send block to all nodes. ")
	// decide how to send, async or sync
	if n.node.NodeNet.CheckHadInputConnects() {
		// can send async. Other nodes can connect to us
		return n.SendBlockToAllASync(newBlock, skipaddr)
	}
	return n.SendBlockToAllSync(newBlock, skipaddr)
}

// Send block in async mode.For case when this node was contacted from outside
// This only sends new block hash to all known nodes and they should contact back to get full body
func (n *communicationManager) SendBlockToAllASync(newBlock *structures.Block, skipaddr net.NodeAddr) error {
	n.logger.Trace.Printf("Send block to all nodes in ASync mode. %x", newBlock.Hash)

	blockshortdata, err := newBlock.GetShortCopy().Serialize()

	if err != nil {
		return err
	}

	for i, node := range n.node.NodeNet.Nodes {
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}

		errc := n.node.NodeClient.SendInv(node, "block", [][]byte{blockshortdata})
		n.node.NodeNet.HookNeworkOperationResult(errc, i) // to know if this node is available

	}
	return nil
}

// Send block in sync mode. For each node firstly request if a node has this block
// If no it eill send a block body
func (n *communicationManager) SendBlockToAllSync(newBlock *structures.Block, skipaddr net.NodeAddr) error {
	n.logger.Trace.Printf("Send block to all nodes in Sync mode. %x", newBlock.Hash)

	blockshortdata, err := newBlock.GetShortCopy().Serialize()

	if err != nil {
		return err
	}

	for i, node := range n.node.NodeNet.Nodes {
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}
		result, err := n.node.NodeClient.SendCheckBlock(node, blockshortdata)

		n.node.NodeNet.HookNeworkOperationResult(err, i) // to know if this node is available

		if err != nil {
			n.logger.Trace.Printf("Error when check if block exists on other node %s for %s", err.Error(), node.NodeAddrToString())
			continue
		}

		if !result.Exists {
			// send full block this this node
			bs, err := newBlock.Serialize()

			if err != nil {
				n.logger.Trace.Printf("Error when serialise a block %s for %x", err.Error(), newBlock.Hash)
				continue
			}
			n.node.NodeClient.SendBlock(node, bs)
		}

	}
	return nil
}

// Check for updates on other nodes
func (n *communicationManager) CheckForChangesOnOtherNodes(lastCheckTime int64) error {

	// get max 10 random nodes where connection was success before
	nodes := n.node.NodeNet.GetConnecttionVerifiedNodeAddresses(10)

	n.logger.Trace.Printf("Commun Man: check from %d nodes", len(nodes))

	// get local blockchain state to send request to other

	myBestHeight, topHashes, err := n.node.NodeBC.GetBCTopState(5)

	if err != nil {
		return err
	}

	for _, node := range nodes {
		n.logger.Trace.Printf("Check node %s", node.NodeAddrToString())
		if node.CompareToAddress(n.node.NodeClient.NodeAddress) {
			continue
		}

		n.pullUpdatesFromNode(node, lastCheckTime, topHashes, myBestHeight)
	}
	return nil
}

// Load updates from a node
func (n *communicationManager) pullUpdatesFromNode(node *net.NodeAddr, lastCheckTime int64, topHashes [][]byte, myBestHeight int) error {
	result, err := n.node.NodeClient.SendGetUpdates(*node, lastCheckTime, myBestHeight, topHashes)

	if err != nil {
		n.node.NodeNet.HookNeworkOperationResultForNode(err, node)
		return err
	}

	err = n.processBlocksFromOtherNode(node, result.Blocks)

	if err != nil {
		return err
	}

	err = n.processTransactionsFromPoolOnOtherNode(node, result.TransactionsInPool)

	if err != nil {
		return err
	}

	err = n.processNodesFromPoolOnOtherNode(node, result.Nodes)

	if err != nil {
		return err
	}

	return nil
}

// Process blocks list received from other node
func (n *communicationManager) processBlocksFromOtherNode(node *net.NodeAddr, blocks [][]byte) error {
	for _, bsdata := range blocks {
		bs, err := structures.NewBlockShortFromBytes(bsdata)

		if err != nil {
			n.logger.Error.Printf("Error when deserialize block info %s", err.Error())
			continue
		}
		// check if block exists
		blockstate, err := n.node.NodeBC.CheckBlockState(bs.Hash, bs.PrevBlockHash)

		if err != nil {
			n.logger.Error.Printf("Error when check block state %s", err.Error())
			continue
		}

		if blockstate == 2 {
			// previous hash is not found. all next blocks will also be missed
			// we don't have good way to process this case
			// we need to go more deeper with requeting of blocks.
			// TODO
			// no solution yet.
			n.logger.Trace.Printf("Previous block not found: %x . Exit blocks loop", bs.PrevBlockHash)
			return nil
		}

		if blockstate == 0 {
			// in this case we can request this block full info

			result, err := n.node.NodeClient.SendGetBlock(*node, bs.Hash)

			if err != nil {
				n.logger.Error.Printf("Error when reuest block body %s", err.Error())
				continue
			}

			n.logger.Trace.Printf("Success loaded block %x", bs.Hash)

			blockstate, _, _, err := n.node.ReceivedFullBlockFromOtherNode(result.Block)

			n.logger.Trace.Printf("adding new block ith state %d", blockstate)
			// state of this adding we don't check. not interesting in this place
			if err != nil {
				n.logger.Error.Printf("Error when adding block body %s", err.Error())
				continue
			}

		}
	}
	return nil
}

// Process blocks list received from other node
func (n *communicationManager) processTransactionsFromPoolOnOtherNode(node *net.NodeAddr, transactions [][]byte) error {
	n.logger.Trace.Printf("Check %d transactions from remote pool", len(transactions))

	if len(transactions) == 0 {
		n.logger.Trace.Printf("Nothing to check")
		return nil
	}

	for _, txID := range transactions {
		// if not exist , request for full body of a TX and add to a pool
		if txe, err := n.node.GetTransactionsManager().GetIfExists(txID); err == nil && txe != nil {
			n.logger.Trace.Printf("TX already exists: %x ", txID)
			continue
		}
		// request this TX and add to the pool
		result, err := n.node.NodeClient.SendGetTransaction(*node, txID)

		if err != nil {
			n.logger.Error.Printf("Error when request for TX from other node %s", err.Error())
			continue
		}
		tx, err := structures.DeserializeTransaction(result.Transaction)

		if err != nil {
			n.logger.Error.Printf("Error when deserialize TX %s", err.Error())
			continue
		}
		n.node.getBlockMakeManager().AddTransactionToPool(tx, lib.TXFlagsExecute)
	}
	return nil
}

// Process nodes addresses list received from other node
func (n *communicationManager) processNodesFromPoolOnOtherNode(node *net.NodeAddr, nodes []net.NodeAddrShort) error {
	n.logger.Trace.Printf("Check %d nodes from remote list", len(nodes))

	if len(nodes) == 0 {
		n.logger.Trace.Printf("Nothing to check")
		return nil
	}

	for _, ns := range nodes {
		addr := net.NodeAddr{}
		addr.Port = ns.Port
		addr.Host = string(ns.Host)

		n.node.checkAddressKnown(addr, false)
	}
	return nil
}
