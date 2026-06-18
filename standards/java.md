# Java — Language and Framework Standards

Apply these rules when working in any Java package in this repository.
This standard is framework-agnostic and covers both Maven and Gradle build tools.

---

## Project Initialisation

**Maven:**
```bash
mvn archetype:generate \
  -DgroupId=com.<owner>.<repo-name> \
  -DartifactId=<repo-name> \
  -DarchetypeArtifactId=maven-archetype-quickstart \
  -DinteractiveMode=false
```

**Gradle:**
```bash
gradle init --type java-application --dsl groovy
```

- Use Java 17 LTS or later — set `<maven.compiler.source>` / `<maven.compiler.target>` (Maven) or `sourceCompatibility` (Gradle) explicitly in the build file.
- Never edit `pom.xml` or `build.gradle` lock files by hand — use the build tool.
- Commit: `chore: scaffold Java project structure`

---

## Build Verification

After any change to Java source, imports, or dependencies — run in this order:

**Maven:**
```bash
mvn clean verify
```

**Gradle:**
```bash
./gradlew build
```

Never claim an implementation is complete without the build passing cleanly.

---

## Verification Gate (build + test)

The build+test pass is the **mandatory gate** at two specific
points in the pipeline. The same command runs in both places.

**Maven:**
```bash
mvn clean verify
```

**Gradle:**
```bash
./gradlew build
```

`mvn clean verify` compiles, runs unit tests, and runs the
verify-phase checks (integration tests, coverage gates) in one
pass; `./gradlew build` does the equivalent for Gradle projects.
The command must exit zero. Any non-zero exit — compilation
failure, failing test, coverage-gate breach surfaced through
`verify`, etc. — **fails** the gate.

### Stack-eligibility pre-check (manifest presence)

The Java gate only applies to a repository when its **build
manifest is present**:

```bash
test -f pom.xml || test -f build.gradle || test -f build.gradle.kts
```

If no Java build manifest is present at the repo root (i.e. this
isn't a Maven/Gradle project), the Java gate is **not eligible**
for this repository. Compliance and the dev session SKIP the gate
with a WARN ("Java manifest `pom.xml`/`build.gradle` not present —
Java gate not applicable to this repo"); the verdict is **not a
fail**.

This handles the legitimate case where a multi-stack repo (e.g.
Java + TypeScript) gets a Feature whose diff touches only files in
a directory that is NOT a Java module — those files are not
verifiable as a Java project and the gate should not pretend
otherwise.

### Dev Session — last step before exit

After the final task commit and before the workflow applies
`in-verification`, the dev session **MUST** run the gate when
eligible (manifest present). On gate failure the dev session does
NOT exit cleanly — it loops back to fix the breakage and re-runs
the gate until it passes. Pushing a broken build or a failing test
suite and signalling completion is forbidden.

The dev session's exit block must state, for each touched stack,
whether the gate ran and what its result was (PASS / FAIL / SKIPPED).
An exit block that omits the gate result is itself a protocol
violation.

### Compliance Verify — first step before any other check

The compliance verifier **MUST** run the gate (when eligible)
before evaluating acceptance criteria, static analysis, or any
other check. On gate failure the verifier emits an immediate FAIL
verdict and short-circuits — ACs cannot be PASS while the build is
broken or the tests fail, regardless of what code inspection
suggests.

The gate's run-and-result is the first item in the compliance
report. Subsequent sections (static analysis, AC table) appear only
when the gate passed or was skipped per the rules below.

### When the toolchain is unavailable

If a Java build manifest IS present but the toolchain is **not** on
the runner's PATH (`mvn` for a Maven project, or a usable JDK / the
`./gradlew` wrapper for a Gradle project), the gate is treated as
**SKIPPED with a WARN** — not as PASS, not as FAIL, not as BLOCKED.
The recipe records:

- a `compliance-warn:v1` comment noting that the Java gate was
  skipped because the toolchain isn't installed on the runner, with
  the exact `which mvn` / `java -version` probe output
- a recommendation to install the JDK + build tool on the runner
  image (via `actions/setup-java@v4` plus Maven/Gradle, or a runner
  image that bundles them) so the gate can actually run on the next
  cycle
- AC verdicts for build / test fields are marked **WARN — skipped**
  rather than PASS or FAIL

Compliance still produces an overall verdict, runs the static
analysis section, and evaluates the AC table. The PR is permitted
to open. CI (`build-and-test.yml` or equivalent) is the
authoritative backstop for actually running the tests once the PR
is open.

This mirrors the Go and TypeScript gates: toolchain absence is a
runner-image problem, not a reason to leave a real Feature in a
permanent stuck-state. The intent is preserved by the "WARN, never
PASS by inspection" rule below.

### What is still forbidden

- **PASS-by-inspection is still forbidden.** Compliance MUST NOT
  emit a PASS verdict for the gate based on diff inspection alone.
  The gate is either PASS (commands ran and exited zero), FAIL
  (commands ran and exited non-zero), or SKIPPED-with-WARN
  (commands could not run). No fourth state.
- **FAIL-by-inspection is still forbidden.** Compliance MUST NOT
  emit FAIL based on a CI run from a closed PR, sibling branch, or
  any commit other than the current branch HEAD. Same trap, opposite
  direction.

---

## Coding Standards

- **Dependency injection** — use constructor injection for all collaborators. Field injection (`@Autowired` on fields) is prohibited. All dependencies must be injectable for testing.
- **Immutability** — prefer `final` fields; make classes and methods `final` unless designed for extension. Use `Collections.unmodifiableList` / `List.copyOf` when returning collection fields.
- **Null safety** — never return `null` from public methods; use `Optional<T>` or throw a typed exception. Annotate parameters and return types with `@NonNull` / `@Nullable` where ambiguity exists.
- **Error handling** — define typed exception classes per domain boundary. Never throw `RuntimeException` or `Exception` directly; never catch-and-swallow (`catch (Exception e) {}`).
- **Constants** — numeric literals and strings with business meaning must be named constants (`static final`). Never hardcode timeouts, retry counts, or thresholds in logic.
- **Time** — never call `LocalDateTime.now()` or `Instant.now()` inside business logic — inject a `Clock` parameter. Store and publish UTC. Use `ChronoUnit` and `Duration` for comparisons.
- **Financial values** — use `java.math.BigDecimal` with explicit `RoundingMode`. Never use `double` or `float` for financial calculations.
- **Sensitive data** — never log subscriber identifiers, balances, or transaction amounts. Never return internal stack traces or exception messages to API callers. Credentials from config files only.
- **Thread safety** — protect shared mutable state with `synchronized`, `ReentrantLock`, or `java.util.concurrent` types. Prefer immutable data structures in concurrent contexts.

---

## Error Handling

- Define typed exception classes per domain boundary — e.g. `InsufficientFundsException`, `SubscriberNotFoundException`.
- Use unchecked exceptions (`RuntimeException` subclasses) for domain errors. Checked exceptions are reserved for recoverable I/O and infrastructure failures.
- Never use string comparison on exception messages — catch the typed class.
- Always log the exception cause when wrapping: `throw new DomainException("context", cause)`.
- `catch (Exception e)` is only permitted at application boundary handlers (e.g. servlet filters, Kafka error handlers) — never in business logic.

---

## Testing

**Framework:** JUnit 5 with Mockito

**Commands:**

Maven:
```bash
mvn test                    # run all tests
mvn test -pl <module>       # specific module
mvn -Dtest=MyTest test      # specific test class
```

Gradle:
```bash
./gradlew test              # run all tests
./gradlew test --tests MyTest  # specific test class
```

**Requirements:**
- Every Java class with business methods must have an accompanying `Test` class (e.g. `UserService` → `UserServiceTest`)
- Classes that only declare interfaces, constants, or DTOs without logic are exempt
- Tests must run and pass — writing without running does not satisfy this rule
- Unit tests must NOT require external services (databases, message brokers) — mock at the boundary

**Test class structure:**
```java
@ExtendWith(MockitoExtension.class)
class UserServiceTest {

    @Mock
    private UserRepository userRepository;

    @InjectMocks
    private UserService userService;

    @Test
    void findUser_existingId_returnsUser() {
        // arrange
        // act
        // assert
    }

    @Test
    void findUser_unknownId_throwsNotFoundException() {
        assertThrows(UserNotFoundException.class,
            () -> userService.findUser("unknown-id"));
    }
}
```

**Test naming:** `methodName_scenario_expectedResult`
e.g. `calculateBalance_insufficientFunds_throwsException`

**Parameterised tests** — required for functions with multiple input/output combinations:
```java
@ParameterizedTest
@MethodSource("provideInvalidInputs")
void validate_invalidInput_throwsValidationException(String input, String expectedCode) {
    var ex = assertThrows(ValidationException.class, () -> service.validate(input));
    assertEquals(expectedCode, ex.getCode());
}
```

**No manual mock initialisation** — always use `@ExtendWith(MockitoExtension.class)` rather
than `MockitoAnnotations.openMocks(this)`. The extension cleans up automatically.

---

## Architecture Boundaries

- Transport handlers (REST controllers, Kafka consumers) must be thin — delegate all logic to services
- No business logic in HTTP controllers or message listener methods
- All database access through repository interfaces — never call JPA/JDBC directly from service classes
- Kafka consumers must delegate to services — no business logic in `@KafkaListener` methods
- Configuration from application properties only — never read `System.getenv()` inside business logic

---

## Dependency Management

**Maven:**
```bash
mvn dependency:tree          # inspect transitive dependencies
mvn versions:display-dependency-updates  # check for updates
```

**Gradle:**
```bash
./gradlew dependencies       # inspect transitive dependencies
```

- Prefer libraries already used in the project over introducing new ones
- Declare test-scoped dependencies with `<scope>test</scope>` (Maven) or `testImplementation` (Gradle) — never in compile scope
- Never suppress compiler warnings with `@SuppressWarnings` without a code comment explaining why
- Pin dependency versions in the parent POM or `gradle.properties` — avoid unbound version ranges

---

## Static Analysis

The compliance-verify skill reads this section to execute the correct toolchain
when verifying a Java Feature. Run these tools in order against the full module tree.
Commands are shown for both Maven and Gradle — use whichever the project uses.

### Native tools — commands

| Tool | Maven command | Gradle command | Notes |
|---|---|---|---|
| SpotBugs | `mvn spotbugs:check` | `./gradlew spotbugsMain` | Bug and correctness analysis |
| Checkstyle | `mvn checkstyle:check` | `./gradlew checkstyleMain` | Coding-standards enforcement |
| PMD | `mvn pmd:check` | `./gradlew pmdMain` | Skip if not configured in build file |
| OWASP Dependency Check | `mvn dependency-check:check` | `./gradlew dependencyCheckAnalyze` | Known CVE scan against declared dependencies |

### Native tools — severity mapping

| Tool | Finding type | Compliance severity |
|---|---|---|
| SpotBugs | `SCARY` or `TROUBLING` category | CRITICAL |
| SpotBugs | `DODGY` / style category | MAJOR |
| Checkstyle | any violation | MINOR |
| PMD | Priority 1–2 (error-prone, security) | CRITICAL |
| PMD | Priority 3–4 (design, best practice) | MAJOR |
| PMD | Priority 5 (code style) | MINOR |
| OWASP Dependency Check | CVSS ≥ 7.0 (High / Critical) | CRITICAL |
| OWASP Dependency Check | CVSS 4.0–6.9 (Medium) | MAJOR |
| OWASP Dependency Check | CVSS < 4.0 (Low) | MINOR |

---

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a Java Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Test Quality Expectations

Coverage numbers alone are not sufficient. The compliance verifier additionally
enforces:

- Tests must assert on the content of returned values and thrown exceptions —
  not merely that an exception was thrown. `assertThrows(SomeException.class, ...)`
  without a follow-up assertion on the exception's message or code does not
  satisfy error-path coverage.
- Parameterised tests are required for methods with two or more distinct input
  scenarios. Repeating nearly identical `@Test` methods rather than a
  `@ParameterizedTest` is a failing pattern.
- At least 50% of test lines must exercise non-trivial logic — conditional branches,
  exception paths, business-rule outcomes. Tests that only instantiate a DTO and
  call a getter do not satisfy the 80% threshold in spirit.

### Java-Specific Enforcement Rules

1. **No `@SuppressWarnings("unchecked")` in test code** — suppressing unchecked
   warnings in test files masks type-safety violations the test is meant to catch.
   Any `@SuppressWarnings("unchecked")` annotation inside a `*Test.java` class
   is a failing pattern.

3. **Business logic must not be in static utility methods** — static methods cannot
   be overridden, mocked, or injected. Any method that contains domain logic
   (branching on business rules, calling repositories, computing financial values)
   must be an instance method on an injectable class. Static utility methods are
   permitted only for pure data transformations with no business-rule branching.

4. **`@ExtendWith(MockitoExtension.class)` is mandatory** — test classes that use
   Mockito must declare `@ExtendWith(MockitoExtension.class)` at the class level.
   Manual `MockitoAnnotations.openMocks(this)` calls in `@BeforeEach` are a
   failing pattern; the extension manages lifecycle automatically.
