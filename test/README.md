# Tests

## E2E mode

The metallb e2e tests are divided into 2 suites:
- validation - verify if the metallb operator has been installed correctly
- functional - verify that the installation of metallb by the operator is working correctly

To run the tests use the following make targets:
- make test-validation - run the validation tests
- make test-functional - run the functional tests
- make test-e2e - run all tests
