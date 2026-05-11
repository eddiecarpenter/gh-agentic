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

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a Java Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Coverage Threshold

≥80% statement coverage is required for every class containing business logic.

**Coverage command:**

Maven (JaCoCo):
```bash
mvn test jacoco:report
```

Gradle (JaCoCo):
```bash
./gradlew test jacocoTestReport
```

Configure JaCoCo to fail the build when coverage drops below 80%:

Maven (`pom.xml`):
```xml
<plugin>
  <groupId>org.jacoco</groupId>
  <artifactId>jacoco-maven-plugin</artifactId>
  <executions>
    <execution>
      <goals><goal>check</goal></goals>
      <configuration>
        <rules>
          <rule>
            <limits>
              <limit>
                <counter>INSTRUCTION</counter>
                <minimum>0.80</minimum>
              </limit>
            </limits>
          </rule>
        </rules>
      </configuration>
    </execution>
  </executions>
</plugin>
```

Any class below 80% instruction coverage fails the compliance check.

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

1. **JaCoCo coverage gate** — every module must be configured with the JaCoCo
   Maven or Gradle plugin at ≥80% instruction coverage. The coverage report
   (`mvn test jacoco:report` / `./gradlew test jacocoTestReport`) must be
   present after the build. A module without JaCoCo configured fails the check.

2. **No `@SuppressWarnings("unchecked")` in test code** — suppressing unchecked
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
