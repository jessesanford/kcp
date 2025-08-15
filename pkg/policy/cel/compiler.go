package cel

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/kcp-dev/kcp/pkg/policy/interfaces"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Compiler compiles CEL expressions
type Compiler struct {
	env *cel.Env
}

// NewCompiler creates a new expression compiler
func NewCompiler(env *cel.Env) *Compiler {
	return &Compiler{
		env: env,
	}
}

// Compile compiles an expression string
func (c *Compiler) Compile(ctx context.Context, expression string) (interfaces.CompiledExpression, error) {
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compilation failed: %w", issues.Err())
	}
	
	prog, err := c.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program creation failed: %w", err)
	}
	
	return &compiledExpression{
		ast:  ast,
		prog: prog,
	}, nil
}

// CompileWithEnv compiles with a specific environment
func (c *Compiler) CompileWithEnv(ctx context.Context, expression string, env *interfaces.Environment) (interfaces.CompiledExpression, error) {
	// Create custom environment
	customEnv, err := c.createCustomEnvironment(env)
	if err != nil {
		return nil, err
	}
	
	ast, issues := customEnv.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compilation failed: %w", issues.Err())
	}
	
	prog, err := customEnv.Program(ast)
	if err != nil {
		return nil, err
	}
	
	return &compiledExpression{
		ast:  ast,
		prog: prog,
	}, nil
}

// Validate checks expression syntax
func (c *Compiler) Validate(ctx context.Context, expression string) error {
	_, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return issues.Err()
	}
	return nil
}

// createCustomEnvironment creates a custom CEL environment
func (c *Compiler) createCustomEnvironment(env *interfaces.Environment) (*cel.Env, error) {
	opts := []cel.EnvOption{
		cel.StdLib(),
		RegisterCustomFunctions(),
	}
	
	// Add variables
	for name, varType := range env.Variables {
		opts = append(opts, cel.Variable(name, getCELType(varType)))
	}
	
	return cel.NewEnv(opts...)
}

// compiledExpression wraps a compiled CEL expression
type compiledExpression struct {
	ast  *cel.Ast
	prog cel.Program
}

// Evaluate runs the expression with variables
func (e *compiledExpression) Evaluate(vars map[string]interface{}) (interface{}, error) {
	out, _, err := e.prog.Eval(vars)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

// Type returns the result type
func (e *compiledExpression) Type() interfaces.ExpressionType {
	return interfaces.TypeBool
}

// Cost returns computational cost
func (e *compiledExpression) Cost() uint64 {
	// Simple cost estimation based on AST complexity
	// In practice, this would be more sophisticated
	return uint64(len(e.ast.GetExpr().String()))
}