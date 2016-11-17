package main

import (
	"SDCA-Makefile/compilationInterface"
	"crypto/tls"
	"fmt"
	"log"
	"path/filepath"
	"git.apache.org/thrift.git/lib/go/thrift"
	"sync"
	"os"
)

var busy []bool
var mutex sync.Mutex
var current_server_id int = 0
var workingDir string

/*
Create thrift transport
 */
func createConnection(transportFactory *thrift.TTransportFactory, addr string, secure bool) (error, *thrift.TTransport) {
	var transport = new(thrift.TTransport)
	var err error

	if secure {
		cfg := new(tls.Config)
		cfg.InsecureSkipVerify = true
		*transport, err = thrift.NewTSSLSocket(addr, cfg)
	} else {
		*transport, err = thrift.NewTSocket(addr)
	}
	if err != nil {
		return err, nil
	}

	*transport = (*transportFactory).GetTransport(*transport)
	defer (*transport).Close()
	if err := (*transport).Open(); err != nil {
		return err, nil
	}

	return nil, transport
}

/*
Open a thrift connection
 */
func open_connection(t *thrift.TTransport) {
	err := (*t).Open()
	if err != nil {
		log.Fatal(err)
	}
}

/*
Close a thrift connection
 */
func close_connection(t *thrift.TTransport) {
	err := (*t).Close()
	if err != nil {
		log.Fatal(err)
	}
}

/*
Send an action to an other host
 */
func handleTarget(transport *thrift.TTransport, protocolFactory thrift.TProtocolFactory, target *Target, serverName string) (err error) {

	// Configuration of the command
	open_connection(transport)
	client := compilationInterface.NewCompilationServiceClientFactory(*transport, protocolFactory)
	command := compilationInterface.NewCommand()
	command.CommandLine = target.lineCommand
	command.WorkingDir = workingDir
	command.ID = target.id

	// Send the command
	status, err := client.ExecuteCommand(command)
	close_connection(transport)
	if err != nil {
		fmt.Println(serverName ," : There was a problem while running target ",target.id,": ", err.Error())
	}
	fmt.Println(serverName ," : Execute target ",target.id," and return status ",status)

	mutex.Lock()
	target.computing = false
	target.done = true
	busy[target.serverId] = false
	defer mutex.Unlock()

	return err
}

/*
Find an available host
 */
func find_available_server() int {
	mutex.Lock()
	var nb_tested_id int = 0
	for nb_tested_id != len(busy) {
		if busy[current_server_id] == false {
			selected_id := current_server_id
			current_server_id = (current_server_id + 1) % len(busy)
			defer mutex.Unlock()
			return selected_id
		}
		current_server_id = (current_server_id + 1) % len(busy)
		nb_tested_id++
	}
	defer mutex.Unlock()
	return -1
}

/*
Main client function
 */
func runClient(transportFactory thrift.TTransportFactory, protocolFactory thrift.TProtocolFactory, secure bool, hosts []string, makefile string) error {
	var servers []*thrift.TTransport

	// Create thrift connection
	for i := 0; i < len(hosts); i++ {
		if err, server := createConnection(&transportFactory, hosts[i], secure); err != nil {
			fmt.Println("There was a problem while connecting to host " + hosts[i])
			log.Fatal(err)
			os.Exit(1) // Exit
		} else {
			servers = append(servers, server)
			busy = append(busy, false)
		}
	}

	// Parse Makefile
	root_target, _ := Parse(makefile)

	// Calculte working directory
	dir, _ := filepath.Abs(filepath.Dir(makefile))
	workingDir = dir

	// Job distribution while the target is not done
	for root_target.done != true {
		var leaf = root_target.Get_Leaf()
		if leaf != nil {
			if id_server := find_available_server(); id_server != -1 {
				if leaf.lineCommand != ""{
					// Execute the node command
					fmt.Println(hosts[id_server], " : Going to execute target ",leaf.id)
					mutex.Lock()
					leaf.computing = true
					leaf.serverId = id_server
					busy[id_server] = true
					mutex.Unlock()
					go handleTarget(servers[id_server], protocolFactory, leaf, hosts[id_server])
				}else{
					// There is no command to execute so this target is done
					fmt.Println("No command for target :", leaf.id)
					mutex.Lock()
					busy[id_server] = false
					leaf.done = true
					mutex.Unlock()
				}

			}
		}
	}

	// End
	return nil
}
