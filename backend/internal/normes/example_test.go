package normes

import "os"

func readExample() ([]byte, error) { return os.ReadFile("../../../docs/normes-exemple.csv") }
