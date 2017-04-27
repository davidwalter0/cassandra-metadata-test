package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/gocql/gocql"

	"net/http"
	"path/filepath"

	trace "github.com/davidwalter0/tracer"
)

var tracer *trace.Tracer
var enable bool

// turn on call trace for debug and testing
func TraceEnvConfig() bool {
	switch strings.ToLower(os.Getenv("WRAP_BUFFER_TRACE_ENABLE")) {
	case "enable", "true", "1", "ok", "ack", "on", "yes":
		return EnableTrace(true)
	case "disable", "false", "0", "nak", "off", "no":
		fallthrough
	default:
		return EnableTrace(false)
	}
}

func EnableTrace(e bool) bool {
	enable = e
	return e
}

func Tracer() *trace.Tracer {
	return tracer
}

func init() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))
	tracer = trace.New()
}

type CqlSession struct {
	*gocql.Session
}

func PivotRoot(rootfs string, removeDir bool) error {
	// pivotDir, err := ioutil.TempDir(rootfs, ".pivot_root")
	// if err != nil {
	// 	return fmt.Errorf("can't create pivot_root dir %s, error %v", pivotDir, err)
	// }

	pivotDir := rootfs
	if err := syscall.PivotRoot(rootfs, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %s", err)
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}

	// path to pivot dir now changed, update
	pivotDir = filepath.Join("/", filepath.Base(pivotDir))
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %s", err)
	}
	// return os.Remove(pivotDir)
	return nil
}

func (c *CqlSession) createTableUsers() {
	text := `
CREATE TABLE users (
firstname text,
lastname text,
age int,
email text,
city text,
PRIMARY KEY (lastname))
`
	c.Query(text)
}

func (c *CqlSession) Query(text string) {
	query := c.Session.Query(text)
	log.Println(query.Exec())
}

func main() {
	var ok bool
	done := make(chan bool)
	go func() {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error while getting current directory.")
			return
		}
		work_directory := cwd + "/work"
		log.Println(work_directory)
		log.Println(os.Mkdir(cwd+"/work", 0700))

		http.Handle("/", http.FileServer(http.Dir(work_directory)))
		port := os.Getenv("PORT")
		host := os.Getenv("HOST")

		if len(port) == 0 {
			fmt.Println("PORT not set, using 8080")
			port = "8080"
		} else {
			fmt.Println("PORT=" + port)
		}

		if len(host) == 0 {
			fmt.Println("HOST not set, default bind all")
			host = "0.0.0.0"
		} else {
			fmt.Println("HOST=" + host)
		}
		listen := host + ":" + port

		fmt.Println("PORT on which  " + ":" + port)
		fmt.Println("HOST interface " + ":" + host)

		fmt.Println("listening on " + listen)
		log.Println(http.ListenAndServe(listen, nil))

		log.Println("Exiting go func...")
	}()

	var keyspace = "demo"
	// flagRF           = flag.Int("rf", 1, "replication factor for test keyspace")
	var flagRF *int = new(int)
	*flagRF = 1
	// connect to the cluster
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Keyspace = "system"
	cluster.Consistency = gocql.LocalQuorum // force it to always select the local
	{
		session, err := cluster.CreateSession()
		if err != nil {
			log.Fatal(err)
		}

		defer session.Close()
		query := session.Query(
			fmt.Sprintf(`CREATE KEYSPACE %s WITH replication = 
                  {
                    'class' : 'SimpleStrategy',
                    'replication_factor' : %d
                  }`, keyspace, *flagRF))
		log.Println(query.Exec())
		// if err != nil {
		// 	log.Fatal(err)
		// }
	}

	cluster.Keyspace = "demo"
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer session.Close()
	c := CqlSession{session}
	c.createTableUsers()
	log.Println("post query")

	// insert a user
	if err := session.Query("INSERT INTO users (lastname, age, city, email, firstname) VALUES ('Jones', 35, 'Austin', 'bob@example.com', 'Bob')").Exec(); err != nil {
		log.Fatal(err)
	}
	// Use select to get the user we just entered
	var firstname, lastname, city, email string
	var age int

	if err := session.Query("SELECT firstname, age FROM users WHERE lastname='Jones'").Scan(&firstname, &age); err != nil {
		log.Fatal(err)
	}
	fmt.Println(firstname, age)

	if err := session.Query(`
       SELECT firstname, lastname, age, email, city 
       FROM users WHERE lastname='Jones'
  `).Scan(&firstname, &lastname, &age, &email, &city); err != nil {
		log.Fatal(err)
	}

	fmt.Println(firstname, lastname, age, email, city)

	// cluster := gocql.NewCluster("localhost")
	// session, err := cluster.CreateSession()
	// defer session.Close()
	DumpKeySpaceMetadata(keyspace, session)
	for {
		select {
		case ok = <-done:
			log.Println("select done...", ok)
		}
	}

	log.Println("exiting...")
}

// type KeyspaceMetadata struct {
//     Name            string
//     DurableWrites   bool
//     StrategyClass   string
//     StrategyOptions map[string]interface{}
//     Tables          map[string]*TableMetadata
// }

func DumpKeySpaceMetadata(keyspace string, session *gocql.Session) {
	// var data interface{}
	var text []byte
	meta, err := session.KeyspaceMetadata(keyspace)

	if err != nil {
		log.Fatal(err)
	}

	text, err = json.MarshalIndent(meta, "", " ")
	fmt.Println(string(text))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(meta.Name, meta.DurableWrites, meta.StrategyClass)
	options := meta.StrategyOptions

	line := ""
	for key, _ := range options {
		line += fmt.Sprintf("%s ", key)
	}
	// log.Println(session.KeyspaceMetadata(keyspace))
	// line = ""
	DumpTableMapMetaData(meta.Tables)
}

// type TableMetadata struct {
//     Keyspace          string
//     Name              string
//     KeyValidator      string
//     Comparator        string
//     DefaultValidator  string
//     KeyAliases        []string
//     ColumnAliases     []string
//     ValueAlias        string
//     PartitionKey      []*ColumnMetadata
//     ClusteringColumns []*ColumnMetadata
//     Columns           map[string]*ColumnMetadata
//     OrderedColumns    []string
// }

func DumpTableMapMetaData(meta map[string]*gocql.TableMetadata) {
	for key, value := range meta {
		log.Println("Table", key)
		DumpTableMetaData(value)
	}
}

func DumpTableMetaData(meta *gocql.TableMetadata) {
	log.Println(
		meta.Keyspace,
		meta.Name,
		meta.KeyValidator,
		meta.DefaultValidator,
	)

	DumpStringArray("KeyAliases", meta.KeyAliases)
	DumpStringArray("ColumnAliases", meta.ColumnAliases)
	log.Println("ValueAlias", meta.ValueAlias)
	DumpColumnMetadata("PartitionKey", meta.PartitionKey)
	DumpColumnMetadata("ClusteringColumns", meta.ClusteringColumns)
	DumpColumnMapMetadata(meta.Columns)
}

func DumpStringArray(name string, array []string) {
	log.Println(name)
	list := ""
	for _, alias := range array {
		list += fmt.Sprintf("%s ", alias)
	}
	log.Println(list)
}

// type ColumnMetadata struct {
//     Keyspace        string
//     Table           string
//     Name            string
//     ComponentIndex  int
//     Kind            ColumnKind
//     Validator       string
//     Type            TypeInfo
//     ClusteringOrder string
//     Order           ColumnOrder
//     Index           ColumnIndexMetadata
// }

func DumpColumnMapMetadata(meta map[string]*gocql.ColumnMetadata) {
	if len(meta) > 0 {
		log.Println("------------------------------------------------------------------------")
		log.Println("DumpColumnMapMetadata")
		log.Println("------------------------------------------------------------------------")
		for _, column := range meta {
			// for key, column := range meta {
			// log.Printf("%s ", key)
			text := fmt.Sprintf("%-20s ",
				column.Keyspace+"."+
					column.Table+"."+
					column.Name,
			)
			log.Printf("%s %3d %v %v %v %v %v %v\n",
				text,
				column.ComponentIndex,
				column.Kind,
				column.Validator,
				column.Type,
				column.ClusteringOrder,
				column.Order,
				column.Index,
			)
		}
		log.Println("------------------------------------------------------------------------")
	}
}

func DumpColumnMetadata(name string, meta []*gocql.ColumnMetadata) {
	var line = ""
	if len(meta) > 0 {
		log.Println("------------------------------------------------------------------------")
		line = fmt.Sprintf("%s ", name)
		for i, column := range meta {
			if i > 1 {
				line += "|"
			}
			line += fmt.Sprintf("%v %v %v",
				column.Keyspace,
				column.Table,
				column.Name,
			)
		}
		log.Println(line)
		for _, column := range meta {
			log.Println(
				column.Keyspace,
				column.Table,
				column.Name,
				column.ComponentIndex,
				column.Kind,
				column.Validator,
				column.Type,
				column.ClusteringOrder,
				column.Order,
				column.Index,
			)
		}
		log.Println("------------------------------------------------------------------------")
	}
}
