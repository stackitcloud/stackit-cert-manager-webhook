# End-to-End (E2E) Conformance Test Suite

This directory contains the cert-manager conformance testing procedures for this project module. It harnesses the `fixture.RunConformance` method to execute conformity verification sequences, ensuring adherence to `cert-manager` established protocols.

## Test Execution Workflow

### Environmental Prerequisite Configuration

For appropriate test initialization within the STACKIT ecosystem, you must configure your local environment and the mock data properly.

1. **Project Identification Parameterization**:
   Configure the unique `projectId` in the [configuration manifest](../testdata/stackit/config.json). Typical configuration appears as:
   ```json
   {
     "projectId": "c242332a-ae82-42e2-80e8-eed338fd2b2f",
     "serviceAccountSecretRef": "stackit-cert-manager-webhook",
     "serviceAccountSecretKey": "sa.json",
     "serviceAccountSecretNamespace": "default"
   }
   ```
   This assumes the specified project exists, and the Service Account key possesses the requisite DNS Admin privileges.

2. **Authentication Key Configuration**:
   The conformance test suite dynamically loads resources from `testdata/stackit/` into its mocked API server. You must create a valid Kubernetes Secret manifest containing your Service Account JSON.

   Generate `secret.yaml` from the provided example:
   ```bash
   export STACKIT_SA_KEY=$(cat /path/to/your/sa.json | base64 -w 0)
   envsubst < ../testdata/stackit/secret.yaml.example > ../testdata/stackit/secret.yaml
   ```

3. **Zone Initialization**:
   Ensure your testing DNS zone exists in your STACKIT project. You can create this via the STACKIT Portal or the STACKIT CLI.

   Declare the testing DNS zone as an environment variable before running the suite:
   ```bash
   export TEST_ZONE_NAME="test-zone.runs.onstackit.cloud"
   ```

### Running the Suite

With prerequisites addressed and the `secret.yaml` populated, proceed to run the conformance test suite:

```bash
make test-e2e-conformance
```