package main

import (
	"bytes"
	"encoding/pem"
	"io/ioutil"
	"os"

	"github.com/Azure/azure-sdk-for-go/management"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/codegangsta/cli"
	"golang.org/x/crypto/pkcs12"

	log "github.com/sirupsen/logrus"
)

var (
	// GitSummary contains version info, provided by govvv at compile time
	GitSummary string
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)
}

// Common CLI flags
var (
	flPackage = cli.StringFlag{
		Name:  "package",
		Usage: "Path of extension package (.zip)"}
	flManifest = cli.StringFlag{
		Name:  "manifest",
		Usage: "Path of extension manifest file (XML output of 'new-extension-manifest')"}
	flMgtURL = cli.StringFlag{
		Name:   "management-url",
		Usage:  "Azure Management URL for a non-public Azure cloud",
		Value:  management.DefaultAzureManagementURL,
		EnvVar: "MANAGEMENT_URL"}
	flStorageRealm = cli.StringFlag{
		Name:   "storage-base-url",
		Usage:  "Azure Storage base URL",
		Value:  storage.DefaultBaseURL,
		EnvVar: "STORAGE_BASE_URL"}
	flSubsID = cli.StringFlag{
		Name:   "subscription-id",
		Usage:  "Subscription ID for the publisher subscription",
		EnvVar: "SUBSCRIPTION_ID",
	}
	flSubsCert = cli.StringFlag{
		Name:   "subscription-cert",
		Usage:  "Path of subscription management certificate (.pem or .pfx) file",
		EnvVar: "SUBSCRIPTION_CERT"}
	flVersion = cli.StringFlag{
		Name:  "version",
		Usage: "Version of the extension package e.g. 1.0.0"}
	flNamespace = cli.StringFlag{
		Name:   "namespace",
		Usage:  "Publisher namespace e.g. Microsoft.Azure.Extensions",
		EnvVar: "EXTENSION_NAMESPACE"}
	flName = cli.StringFlag{
		Name:   "name",
		Usage:  "Name of the extension e.g. FooExtension",
		EnvVar: "EXTENSION_NAME"}
	flStorageAccount = cli.StringFlag{
		Name:  "storage-account",
		Usage: "Name of an existing storage account to be used in uploading the extension package temporarily."}
	flRegion = cli.StringSliceFlag{
		Name:  "region",
		Usage: "List of one or more regions to rollout an extension (e.g. 'Japan East')",
	}
	flJSON = cli.BoolFlag{
		Name:  "json",
		Usage: "Print output as JSON"}
	flIsXMLExtension = cli.BoolFlag{
		Name:  "is-xml-extension",
		Usage: "Set if this is an XML extension, i.e. PaaS"}
)

func main() {
	app := cli.NewApp()
	app.Name = "azure-extensions-cli"
	app.Version = GitSummary
	app.Usage = "This tool is designed for Microsoft internal extension publishers to release, update and manage Virtual Machine extensions."
	app.Authors = []cli.Author{{Name: "Ahmet Alp Balkan", Email: "ahmetb at microsoft döt com"}}
	app.Commands = []cli.Command{
		{Name: "new-extension-manifest",
			Usage:  "Creates an XML file used to publish or update extension.",
			Action: newExtensionManifest,
			Flags: []cli.Flag{
				flMgtURL, flSubsID, flSubsCert, flPackage, flStorageRealm,
				flStorageAccount, flNamespace, flName, flVersion,
				cli.StringFlag{
					Name:  "label",
					Usage: "Human readable name of the extension"},
				cli.StringFlag{
					Name:  "description",
					Usage: "Description of the extension"},
				cli.StringFlag{
					Name:  "eula-url",
					Usage: "URL to the End-User License Agreement page"},
				cli.StringFlag{
					Name:  "privacy-url",
					Usage: "URL to the Privacy Policy page"},
				cli.StringFlag{
					Name:  "homepage-url",
					Usage: "URL to the homepage of the extension"},
				cli.StringFlag{
					Name:  "company",
					Usage: "Human-readable Company Name of the publisher"},
				cli.StringFlag{
					Name:  "supported-os",
					Usage: "Extension platform e.g. 'Linux'"},
			}},
		{Name: "new-extension",
			Usage:  "Creates a new type of extension, not for releasing new versions.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flManifest},
			Action: createExtension},
		{Name: "new-extension-version",
			Usage:  "Publishes a new type of extension internally.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flManifest},
			Action: updateExtension},
		{Name: "promote",
			Usage:  "Promote published internal extension to PROD in one or more locations.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flManifest, flRegion},
			Action: promoteToRegions},
		{Name: "promote-all-regions",
			Usage:  "Promote published extension to all Locations.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flManifest},
			Action: promoteToAllRegions},
		{Name: "list-versions",
			Usage:  "Lists all published extension versions for subscription",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flJSON},
			Action: listVersions},
		{Name: "replication-status",
			Usage:  "Retrieves replication status for an uploaded extension package",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flNamespace, flName, flVersion, flJSON},
			Action: replicationStatus},
		{Name: "unpublish-version",
			Usage:  "Marks the specified version of the extension internal. Does not delete.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flNamespace, flName, flVersion, flIsXMLExtension},
			Action: unpublishVersion},
		{Name: "delete-version",
			Usage:  "Deletes the extension version. It should be unpublished first.",
			Flags:  []cli.Flag{flMgtURL, flSubsID, flSubsCert, flNamespace, flName, flVersion},
			Action: deleteVersion},
	}
	app.RunAndExitOnError()
}

func mkClient(mgtURL, subscriptionID, certFile string) ExtensionsClient {
	b, err := readCert(certFile)
	if err != nil {
		log.Fatalf("Cannot read certificate %s: %v", certFile, err)
	}
	cl, err := NewClient(mgtURL, subscriptionID, b)
	if err != nil {
		log.Fatalf("Cannot create client: %v", err)
	}
	return cl
}

func readCert(certFile string) ([]byte, error) {
	b, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	p, _ := pem.Decode(b)
	if p != nil {
		return b, nil
	}

	pemBlocks, err := pkcs12.ToPEM(b, "")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for _, x := range pemBlocks {
		buf.Write(pem.EncodeToMemory(x))
	}

	return buf.Bytes(), nil
}

func checkFlag(c *cli.Context, fl string) string {
	v := c.String(fl)
	if v == "" {
		log.Fatalf("argument %q must be provided", fl)
	}
	return v
}
