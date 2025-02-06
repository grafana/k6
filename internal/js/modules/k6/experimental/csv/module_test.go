package csv

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"go.k6.io/k6/metrics"

	"go.k6.io/k6/lib"

	"go.k6.io/k6/internal/js/modules/k6/experimental/fs"
	"go.k6.io/k6/lib/fsext"

	"go.k6.io/k6/internal/js/compiler"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
)

// testFilePath holds the path to the test CSV file.
const testFilePath = fsext.FilePathSeparator + "testdata.csv"

// csvTestData is a CSV file that contains test data about
// various composers.
const csvTestData = `lastname,firstname,composer,born,died,dates
Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757
Dorman,Avner,Avner Dorman,1975,,1975–
Still,William Grant,William Grant Still,1895,1978,1895–1978
Bacewicz,Grażyna,Grażyna Bacewicz,1909,1969,1909–1969
Prokofiev,Sergei,Sergei Prokofiev,1891,1953,1891–1953
Lash,Han,Han Lash,1981,,1981–
Franck,César,César Franck,1822,1890,1822–1890
Messiaen,Olivier,Olivier Messiaen,1908,1992,1908–1992
Bellini,Vincenzo,Vincenzo Bellini,1801,1835,1801–1835
Ligeti,György,György Ligeti,1923,2006,1923–2006
`

func TestParserConstructor(t *testing.T) {
	t.Parallel()

	t.Run("constructing a parser without options should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			  const file = await fs.open(%q);
			  const parser = new csv.Parser(file);
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("constructing a parser with valid options should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			  const file = await fs.open(%q);
			  const parser = new csv.Parser(file, { delimiter: ';', skipFirstLine: true, fromLine: 0, toLine: 10 });
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("constructing a parser with both asObjects and skipFirstLine options should fail", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			  const file = await fs.open(%q);
			  const parser = new csv.Parser(file, { delimiter: ';', skipFirstLine: true, asObjects: true });
		`, testFilePath)))

		require.Error(t, err)
	})

	t.Run("constructing a parser with both the asObjects option and fromLine option greater than 0 should fail", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			  const file = await fs.open(%q);
			  const parser = new csv.Parser(file, { delimiter: ';', fromLine: 1, asObjects: true });
		`, testFilePath)))

		require.Error(t, err)
	})

	t.Run("constructing a parser without providing a file instance should fail", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(`
			// Regardless of whether a file is passed, the parser should not be constructed in the VU context.
			const parser = new csv.Parser(null);
		`))

		require.Error(t, err)
		require.Contains(t, err.Error(), "csv Parser constructor takes at least one non-nil source argument")
	})

	t.Run("constructing a parser in VU context should fail", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)
		r.MoveToVUContext(&lib.State{
			Tags: lib.NewVUStateTags(metrics.NewRegistry().RootTagSet()),
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(`
			// Note that we pass an empty object here as opening a real file here would lead to the fs.open call
			// itself to fail (as we're not in the init context).
			const parser = new csv.Parser({}, { delimiter: ';', skipFirstLine: true, fromLine: 0, toLine: 10 });
		`))

		require.Error(t, err)
		require.Contains(t, err.Error(), "csv Parser constructor must be called in the init context")
	})
}

func TestParserNext(t *testing.T) {
	t.Parallel()

	t.Run("next with default options should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file);

			// Parse the header
			let { done, value } = await parser.next();
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}
			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}
			if (JSON.stringify(value) !== JSON.stringify(["lastname", "firstname", "composer", "born", "died", "dates"])) {
				throw new Error("Expected header to be 'lastname,firstname,composer,born,died,dates', but got " + value);
			}

			// Parse the first record
			({ done, value } = await parser.next());
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}
			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}
			if (JSON.stringify(value) !== JSON.stringify(["Scarlatti", "Domenico", "Domenico Scarlatti", "1685", "1757", "1685–1757"])) {
				throw new Error("Expected record to be 'Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757', but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with delimiter options should respect delimiter and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(strings.ReplaceAll(csvTestData, ",", ";")), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { delimiter: ';' });

			// Parse the header
			let { done, value } = await parser.next();
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}

			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}

			if (JSON.stringify(value) !== JSON.stringify(["lastname", "firstname", "composer", "born", "died", "dates"])) {
				throw new Error("Expected header to be 'lastname,firstname,composer,born,died,dates', but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with skipFirstLine options should ignore header and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { skipFirstLine: true });

			// Parse the first record
			const { done, value } = await parser.next();
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}
			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}
			if (JSON.stringify(value) !== JSON.stringify(["Scarlatti", "Domenico", "Domenico Scarlatti", "1685", "1757", "1685–1757"])) {
				throw new Error("Expected record to be 'Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757', but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with fromLine option should start from provided line number and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { fromLine: 9 });

			// Parse the first record from the line at index 9
			const { done, value } = await parser.next();
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}
			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}
			if (JSON.stringify(value) !== JSON.stringify(["Bellini", "Vincenzo", "Vincenzo Bellini", "1801", "1835", "1801–1835"])) {
				throw new Error("Expected record to be 'Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757', but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with skipFirstLine does not impact fromLine and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { skipFirstLine: true, fromLine: 9 });

			// Parse the first record from the line at index 9
			const { done, value } = await parser.next();
			if (done) {
				throw new Error("Expected to read a record, but got done=true");
			}
			if (value.length !== 6) {
				throw new Error("Expected 6 fields, but got " + value.length);
			}
			if (JSON.stringify(value) !== JSON.stringify(["Bellini", "Vincenzo", "Vincenzo Bellini", "1801", "1835", "1801–1835"])) {
				throw new Error("Expected record to be 'Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757', but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with toLine option should end at provided line number and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { toLine: 2 });

			// Ignore the header
			await parser.next();

			// Parse the first record
			await parser.next();

			// Because we asked to parse to line 2 we should effectively parse it
			let { done } = await parser.next();
			if (done) {
				throw new Error("Expected to not be done, but got done=true");
			}

			// Finally because we are past line 2 we should be done
			({ done } = await parser.next());
			if (!done) {
				throw new Error("Expected to be done, but got done=false");
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with skipFirstLine does not impact fromLine and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { skipFirstLine: true, toLine: 2 });

			// Ignore the header
			await parser.next();

			// Parse the first record
			await parser.next();

			// Because we asked to parse until line 2, we should be done, and have reached EOF
			const { done } = await parser.next();
			if (!done) {
				throw new Error("Expected to be done, but got done=false");
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("next with header option should return records as objects and succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file, { asObjects: true });
			let gotParsedCount = 0;

			let { done, value } = await parser.next();
			while (!done) {
				if (typeof value !== 'object' || value === null || Array.isArray(value)) {
					throw new Error("Expected record to be an object, but got " + typeof value);
				}

				if (Object.keys(value).length !== 6) {
					throw new Error("Expected record to have 6 fields, but got " + Object.keys(value).length);
				}

				gotParsedCount++;
				({ done, value } = await parser.next());
			}

			if (gotParsedCount !== 10) {
				throw new Error("Expected to parse 10 records, but got " + gotParsedCount);
			}

		`, testFilePath)))
		require.NoError(t, err)
	})

	t.Run("calling next on a parser that has reached EOF should return done=true and no value", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const parser = new csv.Parser(file);

			// Parse the entire file
			let { done, value } = await parser.next();
			while (!done) {
				({ done, value } = await parser.next());
			}

			// The parser should be done now
			({ done, value } = await parser.next());
			if (!done) {
				throw new Error("Expected to be done, but got done=false");
			}
			if (!Array.isArray(value) || value.length !== 0) {
				throw new Error("Expected value to be a zero length array, but got " + value);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("parse with default options should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file);

			if (csvRecords.length !== 11) {
				throw new Error("Expected 11 records, but got " + csvRecords.length);
			}

			// FIXME @oleiade: Ideally we would check the prototype of the returned object is SharedArray, but
			// the prototype of SharedArray is not exposed to the JS runtime as such at the moment.
			if (csvRecords.constructor !== Array) {
				throw new Error("Expected the result to be a SharedArray, but got " + csvRecords.constructor);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("parse respects the delimiter option and should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(strings.ReplaceAll(csvTestData, ",", ";")), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file, { delimiter: ';' });

			if (csvRecords.length !== 11) {
				throw new Error("Expected 11 records, but got " + csvRecords.length);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("parse respects the skipFirstLine option and should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file, { skipFirstLine: true });

			if (csvRecords.length !== 10) {
				throw new Error("Expected 10 records, but got " + csvRecords.length);
			}

			const wantRecord = ["Scarlatti", "Domenico", "Domenico Scarlatti", "1685", "1757", "1685–1757"];
			if (JSON.stringify(csvRecords[0]) !== JSON.stringify(wantRecord)) {
				throw new Error("Expected first record to be 'Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757', but got " + csvRecords[0]);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("parse respects the fromLine option and should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file, { fromLine: 10 });

			if (csvRecords.length !== 1) {
				throw new Error("Expected 1 records, but got " + csvRecords.length);
			}

			const wantRecord = ["Ligeti","György","György Ligeti","1923","2006","1923–2006"];
			if (JSON.stringify(csvRecords[0]) !== JSON.stringify(wantRecord)) {
				throw new Error("Expected first record to be 'Ligeti,György,György Ligeti,1923,2006,1923–2006', but got " + csvRecords[0]);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("parse respects the toLine option and should succeed", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file, { toLine: 2 });

			if (csvRecords.length !== 3) {
				throw new Error("Expected 3 records, but got " + csvRecords.length);
			}

			const wantRecord = ["Dorman","Avner","Avner Dorman","1975","","1975–"];
			if (JSON.stringify(csvRecords[2]) !== JSON.stringify(wantRecord)) {
				throw new Error("Expected first record to be 'Dorman,Avner,Avner Dorman,1975,,1975–', but got " + csvRecords[1]);
			}
		`, testFilePath)))

		require.NoError(t, err)
	})

	t.Run("parse respects the header option, returns records as objects and succeeds", func(t *testing.T) {
		t.Parallel()

		r, err := newConfiguredRuntime(t)
		require.NoError(t, err)

		// Ensure the testdata.csv file is present on the test filesystem.
		r.VU.InitEnvField.FileSystems["file"] = newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, testFilePath, []byte(csvTestData), 0o644)
		})

		_, err = r.RunOnEventLoop(wrapInAsyncLambda(fmt.Sprintf(`
			const file = await fs.open(%q);
			const csvRecords = await csv.parse(file, { asObjects: true });

			if (csvRecords.length !== 10) {
				throw new Error("Expected 10 records, but got " + csvRecords.length);
			}

			for (const record of csvRecords) {
				if (typeof record !== 'object' || record === null || Array.isArray(record)) {
					throw new Error("Expected record to be an object, but got " + typeof record);
				}

				if (Object.keys(record).length !== 6) {
					throw new Error("Expected record to have 6 fields, but got " + Object.keys(record).length);
				}
			}
		`, testFilePath)))
		require.NoError(t, err)
	})
}

const initGlobals = `
	globalThis.fs = require("k6/experimental/fs");
	globalThis.csv = require("k6/experimental/csv");
`

func newConfiguredRuntime(t testing.TB) (*modulestest.Runtime, error) {
	runtime := modulestest.NewRuntime(t)

	modules := map[string]interface{}{
		"k6/experimental/fs":  fs.New(),
		"k6/experimental/csv": New(),
	}

	err := runtime.SetupModuleSystem(modules, nil, compiler.New(runtime.VU.InitEnv().Logger))
	if err != nil {
		return nil, err
	}

	// Set up the VU environment with an in-memory filesystem and a CWD of "/".
	runtime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{
		"file": fsext.NewMemMapFs(),
	}
	runtime.VU.InitEnvField.CWD = &url.URL{Scheme: "file"}

	// Ensure the `fs` module is available in the VU's runtime.
	_, err = runtime.VU.Runtime().RunString(initGlobals)

	return runtime, err
}

// newTestFs is a helper function that creates a new in-memory file system and calls the provided
// function with it. The provided function is expected to use the file system to create files and
// directories.
func newTestFs(t *testing.T, fn func(fs fsext.Fs) error) fsext.Fs {
	t.Helper()

	filesystem := fsext.NewMemMapFs()

	err := fn(filesystem)
	if err != nil {
		t.Fatal(err)
	}

	return filesystem
}

// wrapInAsyncLambda is a helper function that wraps the provided input in an async lambda. This
// makes the use of `await` statements in the input possible.
func wrapInAsyncLambda(input string) string {
	// This makes it possible to use `await` freely on the "top" level
	return "(async () => {\n " + input + "\n })()"
}
