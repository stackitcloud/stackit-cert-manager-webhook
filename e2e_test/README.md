# End-to-End (E2E) Test Suite

This repository segment encapsulates the comprehensive E2E testing procedures for this project module. It harnesses the `fixture.RunConformance` method to execute conformity verification sequences, ensuring adherence to `cert-manager` established protocols.

## Test Execution Workflow

### Environmental Prerequisite Configuration:
For appropriate test initialization within the STACKIT ecosystem, it is imperative to align the environment with 
the predetermined specifications. STACKIT dictates a structural model where a parent project umbrellas various 
resource entities, inclusive of DNS zones.

1. **Project Identification Parameterization**:   
Configure the unique `project_id` in the [configuration manifest](../testdata/stackit/config.json). Typical configuration appears as:
    ```json
    {
      "projectId": "c242332a-ae82-42e2-80e8-eed338fd2b2f",
      "authTokenSecretNamespace": "default"
    }
    ```

    This instantiation assumes the existence of the specified project, and the associated authentication 
    token possesses requisite privileges for project and zone access.
2. **Authentication Token Configuration**:    
Establish an environment variable for the authentication token, duly vested with CRUD permissions for DNS zones:
    ```bash
    export STACKIT_TOKEN="<your-token>"
    ```
3. **Zone Initialization**:   
Declare the testing DNS zone. Ensure project_id consistency:
    ```bash
    export TEST_ZONE_NAME="test-zone.runs.onstackit.cloud"
    ```
    Invoke the following HTTP request to either instantiate a fresh zone or validate the existing one:
    ```bash
    curl --location "https://dns.api.stackit.cloud/v1/projects/c242332a-ae82-42e2-80e8-eed338fd2b2f/zones" \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $AUTHENTICATION_TOKEN" \
    --data '{
        "name": "cert manager e2e test",
        "dnsName": "$TEST_ZONE_NAME" 
    }'
    ```
    Post successful invocation, validate zone ownership. For pre-existing zones, consider a unique zone parameter.

### Environmental Prerequisite Configuration:
With prerequisites addressed, proceed to run the entire E2E test suite:
```bash
make test-e2e
```
