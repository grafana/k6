package csv

import (
	"io"
	"strings"
	"testing"

	"gopkg.in/guregu/null.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReaderFrom(t *testing.T) {
	t.Parallel()

	t.Run("instantiating a new reader with a nil io.Reader should fail", func(t *testing.T) {
		t.Parallel()

		_, err := NewReaderFrom(nil, options{})
		require.Error(t, err)
	})

	t.Run("instantiating a new reader with the fromLine option less than 0 should fail", func(t *testing.T) {
		t.Parallel()

		_, err := NewReaderFrom(
			strings.NewReader("lastname,firstname,composer,born,died,dates\n"),
			options{FromLine: null.NewInt(-1, true)},
		)
		require.Error(t, err)
	})

	t.Run("instantiating a new reader with the toLine option less than 0 should fail", func(t *testing.T) {
		t.Parallel()

		_, err := NewReaderFrom(
			strings.NewReader("lastname,firstname,composer,born,died,dates\n"),
			options{ToLine: null.NewInt(-1, true)},
		)
		require.Error(t, err)
	})

	t.Run("instantiating a new reader with fromLine greater or equal to toLine should fail", func(t *testing.T) {
		t.Parallel()

		_, err := NewReaderFrom(
			strings.NewReader("lastname,firstname,composer,born,died,dates\n"),
			options{FromLine: null.NewInt(4, true), ToLine: null.NewInt(1, true)},
		)
		require.Error(t, err)
	})

	t.Run("skipFirstLine option skips first line and succeeds", func(t *testing.T) {
		t.Parallel()

		const csvTestData = "lastname,firstname,composer,born,died,dates\n" +
			"Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757\n"

		r, err := NewReaderFrom(
			strings.NewReader(csvTestData),
			options{SkipFirstLine: true},
		)
		require.NoError(t, err)

		records, err := r.csv.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"Scarlatti", "Domenico", "Domenico Scarlatti", "1685", "1757", "1685–1757"}, records)
	})

	t.Run("fromLine option move reading head forward and succeeds", func(t *testing.T) {
		t.Parallel()

		const csvTestData = "lastname,firstname,composer,born,died,dates\n" +
			"Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757\n" +
			"Dorman,Avner,Avner Dorman,1975,,1975–\n" +
			"Still,William Grant,William Grant Still,1895,1978,1895–1978\n"

		r, err := NewReaderFrom(
			strings.NewReader(csvTestData),
			options{FromLine: null.NewInt(2, true)},
		)
		require.NoError(t, err)

		records, err := r.csv.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"Dorman", "Avner", "Avner Dorman", "1975", "", "1975–"}, records)
	})

	t.Run("fromLine option supersedes skipFirstLine option and succeeds", func(t *testing.T) {
		t.Parallel()

		const csvTestData = "lastname,firstname,composer,born,died,dates\n" +
			"Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757\n" +
			"Dorman,Avner,Avner Dorman,1975,,1975–\n" +
			"Still,William Grant,William Grant Still,1895,1978,1895–1978\n"

		r, err := NewReaderFrom(
			strings.NewReader(csvTestData),
			options{SkipFirstLine: true, FromLine: null.NewInt(2, true)},
		)
		require.NoError(t, err)

		records, err := r.csv.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"Dorman", "Avner", "Avner Dorman", "1975", "", "1975–"}, records)
	})
}

func TestReader_Read(t *testing.T) {
	t.Parallel()

	t.Run("default behavior should return all lines and succeed", func(t *testing.T) {
		t.Parallel()

		const csvTestData = "lastname,firstname,composer,born,died,dates\n" +
			"Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757\n"

		r, err := NewReaderFrom(
			strings.NewReader(csvTestData),
			options{},
		)
		require.NoError(t, err)

		// Parsing gotHeader should succeed
		gotHeader, err := r.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"lastname", "firstname", "composer", "born", "died", "dates"}, gotHeader)

		// Parsing first line should succeed
		gotFirstLine, err := r.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"Scarlatti", "Domenico", "Domenico Scarlatti", "1685", "1757", "1685–1757"}, gotFirstLine)

		// As we've reached EOF, we should get EOF
		_, err = r.Read()
		require.Error(t, err)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("toLine option returns EOF when toLine option is reached and succeeds", func(t *testing.T) {
		t.Parallel()

		const csvTestData = "lastname,firstname,composer,born,died,dates\n" +
			"Scarlatti,Domenico,Domenico Scarlatti,1685,1757,1685–1757\n" +
			"Dorman,Avner,Avner Dorman,1975,,1975–\n" +
			"Still,William Grant,William Grant Still,1895,1978,1895–1978\n"

		r, err := NewReaderFrom(
			strings.NewReader(csvTestData),
			options{ToLine: null.NewInt(2, true)},
		)
		require.NoError(t, err)

		// Parsing header should succeed
		_, err = r.Read()
		require.NoError(t, err)

		// Parsing first line should succeed
		_, err = r.Read()
		require.NoError(t, err)

		// Parsing second line should succeed
		_, err = r.Read()
		require.NoError(t, err)

		// As we've reached `toLine`, we should get EOF
		_, err = r.Read()
		require.Error(t, err)
		require.ErrorIs(t, err, io.EOF)
	})
}
