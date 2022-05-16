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
	LET
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

	RETURN
	TYPEOF
	DELETE
	SWITCH

	DEFAULT
	FINALLY

	FUNCTION
	CONTINUE
	DEBUGGER

	INSTANCEOF
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
	CONST:                       "const",
	WHILE:                       "while",
	BREAK:                       "break",
	CATCH:                       "catch",
	THROW:                       "throw",
	RETURN:                      "return",
	TYPEOF:                      "typeof",
	DELETE:                      "delete",
	SWITCH:                      "switch",
	DEFAULT:                     "default",
	FINALLY:                     "finally",
	FUNCTION:                    "function",
	CONTINUE:                    "continue",
	DEBUGGER:                    "debugger",
	INSTANCEOF:                  "instanceof",
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
		token:         KEYWORD,
		futureKeyword: true,
	},
	"enum": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"export": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"extends": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"import": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"super": {
		token:         KEYWORD,
		futureKeyword: true,
	},
	"implements": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
	"interface": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
	"let": {
		token:  LET,
		strict: true,
	},
	"package": {
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
	},
	"static": {
		token:         KEYWORD,
		futureKeyword: true,
		strict:        true,
	},
}
