package main

import (
	"bytes"
	"text/template"

	"github.com/codegangsta/cli"
	log "github.com/sirupsen/logrus"
)

func unpublishVersion(c *cli.Context) {
	p := struct {
		Namespace, Name, Version string
	}{
		Namespace: checkFlag(c, flNamespace.Name),
		Name:      checkFlag(c, flName.Name),
		Version:   checkFlag(c, flVersion.Name)}

	isXMLExtension := c.Bool(flIsXMLExtension.Name)
	buf := bytes.NewBufferString(`<?xml version="1.0" encoding="utf-8" ?>
<ExtensionImage xmlns="http://schemas.microsoft.com/windowsazure"  xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
  <!-- WARNING: Ordering of fields matter in this file. -->
  <ProviderNameSpace>{{.Namespace}}</ProviderNameSpace>
  <Type>{{.Name}}</Type>
  <Version>{{.Version}}</Version>
  <IsInternalExtension>true</IsInternalExtension>
`)

	// All extension should be a JSON extension.  The biggest offenders are
	// PaaS extensions.
	if !isXMLExtension {
		buf.WriteString("<IsJsonExtension>true</IsJsonExtension>")
	}

	buf.WriteString("</ExtensionImage>")
	tpl, err := template.New("unregisterManifest").Parse(buf.String())
	if err != nil {
		log.Fatalf("template parse error: %v", err)
	}

	var b bytes.Buffer
	if err = tpl.Execute(&b, p); err != nil {
		log.Fatalf("template execute error: %v", err)
	}

	cl := mkClient(checkFlag(c, flMgtURL.Name), checkFlag(c, flSubsID.Name), checkFlag(c, flSubsCert.Name))
	op, err := cl.UpdateExtension(b.Bytes())
	if err != nil {
		log.Fatalf("UpdateExtension failed: %v", err)
	}
	lg := log.WithField("x-ms-operation-id", op)
	lg.Info("UpdateExtension operation started.")
	if err := cl.WaitForOperation(op); err != nil {
		lg.Fatalf("UpdateExtension failed: %v", err)
	}
	lg.Info("UpdateExtension operation finished.")
}
