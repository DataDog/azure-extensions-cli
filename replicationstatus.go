package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
)

func replicationStatus(c *cli.Context) {
	cl := mkClient(checkFlag(c, flMgtURL.Name), checkFlag(c, flSubsID.Name), checkFlag(c, flSubsCert.Name))
	ns, name, version := checkFlag(c, flNamespace.Name), checkFlag(c, flName.Name), checkFlag(c, flVersion.Name)
	json := c.Bool(flJSON.Name)
	log.Debug("Requesting replication status.")
	rs, err := cl.GetReplicationStatus(ns, name, version)
	if err != nil {
		log.Fatalf("Cannot fetch replication status: %v", err)
	}

	var f func(_ ReplicationStatusResponse) error
	if json {
		f = printAsJSON
	} else {
		f = printAsTable
	}
	if err := f(rs); err != nil {
		log.Fatal(err)
	}
}

func printAsJSON(r ReplicationStatusResponse) error {
	b, err := json.MarshalIndent(r.Statuses, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format as json: %+v", err)
	}
	fmt.Fprintf(os.Stdout, "%s", string(b))
	return nil
}

func printAsTable(r ReplicationStatusResponse) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Location", "Status"})
	data := [][]string{}
	for _, s := range r.Statuses {
		data = append(data, []string{s.Location, s.Status})
	}
	table.AppendBulk(data)
	table.Render()
	return nil
}
