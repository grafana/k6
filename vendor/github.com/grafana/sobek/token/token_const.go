package token

const (
	_ Token = iota

	ILLEGAL
	EOF
	COMMENT

	STRING
	NUMBER

	PLUS      // +
	MINUS     // -
	MULTIPLY  // *
	EXPONENT  // **
	SLASH     // /
	REMAINDER // %

	AND                  // &
	OR                   // |
	EXCLUSIVE_OR         // ^
	SHIFT_LEFT           // <<
	SHIFT_RIGHT          // >>
	UNSIGNED_SHIFT_RIGHT // >>>

	ADD_ASSIGN       // +=
	SUBTRACT_ASSIGN  // -=
	MULTIPLY_ASSIGN  // *=
	EXPONENT_ASSIGN  // **=
	QUOTIENT_ASSIGN  // /=
	REMAINDER_ASSIGN // %=

	AND_ASSIGN                  // &=
	OR_ASSIGN                   // |=
	EXCLUSIVE_OR_ASSIGN         // ^=
	SHIFT_LEFT_ASSIGN           // <<=
	SHIFT_RIGHT_ASSIGN          // >>=
	UNSIGNED_SHIFT_RIGHT_ASSIGN // >>>=

	LOGICAL_AND // &&
	LOGICAL_OR  // ||
	COALESCE    // ??
	INCREMENT   // ++
	DECREMENT   // --

	EQUAL        // ==
	STRICT_EQUAL // ===
	LESS         // <
	GREATER      // >
	ASSIGN       // =
	NOT          // !

	BITWISE_NOT // ~

	NOT_EQUAL        // !=
	STRICT_NOT_EQUAL // !==
	LESS_OR_EQUAL    // <=
	GREATER_OR_EQUAL // >=

	LEFT_PARENTHESIS // (
	LEFT_BRACKET     // [
	LEFT_BRACE       // {
	COMMA            // ,
	PERIOD           // .

	RIGHT_PARENTHESIS // )
	RIGHT_BRACKET     // ]
	RIGHT_BRACE       // }
	SEMICOLON         // ;
	COLON             // :
	QUESTION_MARK     // ?
	QUESTION_DOT      // ?.
	ARROW             // =>
	ELLIPSIS          // ...
	BACKTICK          // `

	PRIVATE_IDENTIFIER

	// tokens below (and only them) are syntactically valid identifiers

	IDENTIFIER
	KEYWORD
	BOOLEAN
	NULL

	IF
	IN
	OF
	DO

	VAR
	FOR
	NEW
	TRY

	THIS
	ELSE
	CASE
	VOID
	WITH

	CONST
	WHILE
	BREAK
	CATCH
	THROW
	CLASS
	SUPER

	RETURN
	TYPEOF
	DELETE
	SWITCH

	DEFAULT
	FINALLY
	EXTENDS

	FUNCTION
	CONTINUE
	DEBUGGER

	INSTANCEOF
	// ES6 Modules
	EXPORT
	IMPORT

	ESCAPED_RESERVED_WORD
	// Non-reserved keywords below

	LET
	STATIC
	ASYNC
	AWAIT
	YIELD
)

var token2string = [...]string{
	ILLEGAL:                     "ILLEGAL",
	EOF:                         "EOF",
	COMMENT:                     "COMMENT",
	KEYWORD:                     "KEYWORD",
	STRING:                      "STRING",
	BOOLEAN:                     "BOOLEAN",
	NULL:                        "NULL",
	NUMBER:                      "NUMBER",
	IDENTIFIER:                  "IDENTIFIER",
	PRIVATE_IDENTIFIER:          "PRIVATE_IDENTIFIER",
	PLUS:                        "+",
	MINUS:                       "-",
	EXPONENT:                    "**",
	MULTIPLY:                    "*",
	SLASH:                       "/",
	REMAINDER:                   "%",
	AND:                         "&",
	OR:                          "|",
	EXCLUSIVE_OR:                "^",
	SHIFT_LEFT:                  "<<",
	SHIFT_RIGHT:                 ">>",
	UNSIGNED_SHIFT_RIGHT:        ">>>",
	ADD_ASSIGN:                  "+=",
	SUBTRACT_ASSIGN:             "-=",
	MULTIPLY_ASSIGN:             "*=",
	EXPONENT_ASSIGN:             "**=",
	QUOTIENT_ASSIGN:             "/=",
	REMAINDER_ASSIGN:            "%=",
	AND_ASSIGN:                  "&=",
	OR_ASSIGN:                   "|=",
	EXCLUSIVE_OR_ASSIGN:         "^=",
	SHIFT_LEFT_ASSIGN:           "<<=",
	SHIFT_RIGHT_ASSIGN:          ">>=",
	UNSIGNED_SHIFT_RIGHT_ASSIGN: ">>>=",
	LOGICAL_AND:                 "&&",
	LOGICAL_OR:                  "||",
	COALESCE:                    "??",
	INCREMENT:                   "++",
	DECREMENT:                   "--",
	EQUAL:                       "==",
	STRICT_EQUAL:                "===",
	LESS:                        "<",
	GREATER:                     ">",
	ASSIGN:                      "=",
	NOT:                         "!",
	BITWISE_NOT:                 "~",
	NOT_EQUAL:                   "!=",
	STRICT_NOT_EQUAL:            "!==",
	LESS_OR_EQUAL:               "<=",
	GREATER_OR_EQUAL:            ">=",
	LEFT_PARENTHESIS:            "(",
	LEFT_BRACKET:                "[",
	LEFT_BRACE:                  "{",
	COMMA:                       ",",
	PERIOD:                      ".",
	RIGHT_PARENTHESIS:           ")",
	RIGHT_BRACKET:               "]",
	RIGHT_BRACE:                 "}",
	SEMICOLON:                   ";",
	COLON:                       ":",
	QUESTION_MARK:               "?",
	QUESTION_DOT:                "?.",
	ARROW:                       "=>",
	ELLIPSIS:                    "...",
	BACKTICK:                    "`",
	IF:                          "if",
	IN:                          "in",
	OF:                          "of",
	DO:                          "do",
	VAR:                         "var",
	LET:                         "let",
	FOR:                         "for",
	NEW:                         "new",
	TRY:                         "try",
	THIS:                        "this",
	ELSE:                        "else",
	CASE:                        "case",
	VOID:                        "void",
	WITH:                        "with",
	ASYNC:                       "async",
	AWAIT:                       "await",
	YIELD:                       "yield",
	CONST:                       "const",
	WHILE:                       "while",
	BREAK:                       "break",
	CATCH:                       "catch",
	THROW:                       "throw",
	CLASS:                       "class",
	SUPER:                       "super",
	RETURN:                      "return",
	TYPEOF:                      "typeof",
	DELETE:                      "delete",
	SWITCH:                      "switch",
	STATIC:                      "static",
	DEFAULT:                     "default",
	FINALLY:                     "finally",
	EXTENDS:                     "extends",
	FUNCTION:                    "function",
	CONTINUE:                    "continue",
	DEBUGGER:                    "debugger",
	INSTANCEOF:                  "instanceof",
	EXPORT:                      "export",
	IMPORT:                      "import",
}

var keywordTable = map[string]_keyword{
	"if": {
		token: IF,
	},
	"in": {
		token: IN,
	},
	"do": {
		token: DO,
	},
	"var": {
		token: VAR,
	},
	"for": {
		token: FOR,
	},
	"new": {
		token: NEW,
	},
	"try": {
		token: TRY,
	},
	"this": {
		token: THIS,
	},
	"else": {
		token: ELSE,
	},
	"case": {
		token: CASE,
	},
	"void": {
		token: VOID,
	},
	"with": {
		token: WITH,
	},
	"async": {
		token: ASYNC,
	},
	"while": {
		token: WHILE,
	},
	"break": {
		token: BREAK,
	},
	"catch": {
		token: CATCH,
	},
	"throw": {
		token: THROW,
	},
	"return": {
		token: RETURN,
	},
	"typeof": {
		token: TYPEOF,
	},
	"delete": {
		token: DELETE,
	},
	"switch": {
		token: SWITCH,
	},
	"default": {
		token: DEFAULT,
	},
	"finally": {
		token: FINALLY,
	},
	"function": {
		token: FUNCTION,
	},
	"continue": {
		token: CONTINUE,
	},
	"debugger": {
		token: DEBUGGER,
	},
	"instanceof": {
		token: INSTANCEOF,
	},
	"const": {
		token: CONST,
	},
	"class": {
		token: CLASS,
	},
	"enum": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"import": {
		token: IMPORT,
	},
	"extends": {
		token: EXTENDS,
	},
	"export": {
		token: EXPORT,
	},
	"super": {
		token: SUPER,
	},
	/*
		"implements": {
			token:         KEYWORD,
			futureKeyword: true,
			strict:        true,
		},
		"interface": {
			token:         KEYWORD,
			futureKeyword: true,
			strict:        true,
		},*/
	"let": {
		token:  LET,
		strict: true,
	},
	/*"package": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
	"private": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
	"protected": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
	"public": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},*/
	"static": {
		token:  STATIC,
		strict: true,
	},
	"await": {
		token: AWAIT,
	},
	"yield": {
		token: YIELD,
	},
	"false": {
		token: BOOLEAN,
	},
	"true": {
		token: BOOLEAN,
	},
	"null": {
		token: NULL,
	},
}
