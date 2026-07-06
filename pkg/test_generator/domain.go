package testgenerator

import "gopkg.in/yaml.v3"

type Domain interface {
	MergeAllOf(domain Domain) Domain
	Parse(node yaml.Node) error
}

func Parse(node yaml.Node) error {

	return nil
}
