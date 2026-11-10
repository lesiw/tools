package a

import "fmt"

func f(err error) error {
	if err == nil {
		return err // want `err is nil here, so the return is nil`
	}
	return nil
}

func g(err error) error {
	if err != nil {
		return err
	}
	return nil
}

func h(err error) error {
	if err == nil {
		return nil
	}
	return nil
}

func i(err, other error) error {
	if err == nil {
		return other
	}
	return err
}

func k(err error) (int, error) {
	if err == nil {
		return 0, err
	}
	return 1, err
}

func l(err error) error {
	if err == nil {
		err = fmt.Errorf("still nil")
		return err
	}
	return err
}

func j(err error, target error) error {
	if err == target {
		return err
	}
	return nil
}

type node struct{}

func (n *node) copy() *node {
	if n == nil {
		return n
	}
	return &node{}
}
