package daggenerate

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"

	rice "github.com/GeertJohan/go.rice"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// Generate outputs a DAG
func Generate(source string, destination string, name string, dryrun bool, out io.Writer) error {

	// Source
	sourcesBox, err := rice.FindBox("sources")
	if err != nil {
		log.Fatal(err)
	}
	s, err := sourcesBox.String(source + ".txt")
	if err != nil {
		log.Fatal(err)
	}

	// Destination
	destinationsBox, err := rice.FindBox("destinations")
	if err != nil {
		log.Fatal(err)
	}
	d, err := destinationsBox.String(destination + ".txt")
	if err != nil {
		log.Fatal(err)
	}

	result := fmt.Sprintf("%s%s", string(s), string(d))

	fmt.Println(result)

	// Write file if dryrun is false
	if dryrun {
		fmt.Println("Dry run. File not written.")
	} else {
		err := ioutil.WriteFile(name+"_dag.py", []byte(result), 0644)
		check(err)
		fmt.Println("Created " + name + "_dag.py")
	}

	return nil
}
