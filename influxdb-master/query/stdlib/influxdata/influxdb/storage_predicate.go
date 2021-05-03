package influxdb

import (
	"fmt"

	"github.com/influxdata/flux/ast"
	"github.com/influxdata/flux/semantic"
	"github.com/influxdata/influxdb/v2/models"
	"github.com/influxdata/influxdb/v2/storage/reads/datatypes"
	"github.com/pkg/errors"
)

// ToStoragePredicate will convert a FunctionExpression into a predicate that can be
// sent down to the storage layer.
func ToStoragePredicate(n semantic.Expression, objectName string) (*datatypes.Predicate, error) {
	root, err := toStoragePredicateHelper(n, objectName)
	if err != nil {
		return nil, err
	}

	return &datatypes.Predicate{
		Root: root,
	}, nil
}

func mergePredicates(op ast.LogicalOperatorKind, predicates ...*datatypes.Predicate) (*datatypes.Predicate, error) {
	if len(predicates) == 0 {
		return nil, errors.New("at least one predicate is needed")
	}

	var value datatypes.Node_Logical
	switch op {
	case ast.AndOperator:
		value = datatypes.LogicalAnd
	case ast.OrOperator:
		value = datatypes.LogicalOr
	default:
		return nil, fmt.Errorf("unknown logical operator %v", op)
	}

	// Nest the predicates backwards. This way we get a tree like this:
	// a AND (b AND c)
	root := predicates[len(predicates)-1].Root
	for i := len(predicates) - 2; i >= 0; i-- {
		root = &datatypes.Node{
			NodeType: datatypes.NodeTypeLogicalExpression,
			Value:    &datatypes.Node_Logical_{Logical: value},
			Children: []*datatypes.Node{
				predicates[i].Root,
				root,
			},
		}
	}
	return &datatypes.Predicate{
		Root: root,
	}, nil
}

func toStoragePredicateHelper(n semantic.Expression, objectName string) (*datatypes.Node, error) {
	switch n := n.(type) {
	case *semantic.LogicalExpression:
		left, err := toStoragePredicateHelper(n.Left, objectName)
		if err != nil {
			return nil, errors.Wrap(err, "left hand side")
		}
		right, err := toStoragePredicateHelper(n.Right, objectName)
		if err != nil {
			return nil, errors.Wrap(err, "right hand side")
		}
		children := []*datatypes.Node{left, right}
		switch n.Operator {
		case ast.AndOperator:
			return &datatypes.Node{
				NodeType: datatypes.NodeTypeLogicalExpression,
				Value:    &datatypes.Node_Logical_{Logical: datatypes.LogicalAnd},
				Children: children,
			}, nil
		case ast.OrOperator:
			return &datatypes.Node{
				NodeType: datatypes.NodeTypeLogicalExpression,
				Value:    &datatypes.Node_Logical_{Logical: datatypes.LogicalOr},
				Children: children,
			}, nil
		default:
			return nil, fmt.Errorf("unknown logical operator %v", n.Operator)
		}
	case *semantic.BinaryExpression:
		left, err := toStoragePredicateHelper(n.Left, objectName)
		if err != nil {
			return nil, errors.Wrap(err, "left hand side")
		}
		right, err := toStoragePredicateHelper(n.Right, objectName)
		if err != nil {
			return nil, errors.Wrap(err, "right hand side")
		}
		children := []*datatypes.Node{left, right}
		op, err := toComparisonOperator(n.Operator)
		if err != nil {
			return nil, err
		}
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeComparisonExpression,
			Value:    &datatypes.Node_Comparison_{Comparison: op},
			Children: children,
		}, nil
	case *semantic.StringLiteral:
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeLiteral,
			Value: &datatypes.Node_StringValue{
				StringValue: n.Value,
			},
		}, nil
	case *semantic.IntegerLiteral:
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeLiteral,
			Value: &datatypes.Node_IntegerValue{
				IntegerValue: n.Value,
			},
		}, nil
	case *semantic.BooleanLiteral:
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeLiteral,
			Value: &datatypes.Node_BooleanValue{
				BooleanValue: n.Value,
			},
		}, nil
	case *semantic.FloatLiteral:
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeLiteral,
			Value: &datatypes.Node_FloatValue{
				FloatValue: n.Value,
			},
		}, nil
	case *semantic.RegexpLiteral:
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeLiteral,
			Value: &datatypes.Node_RegexValue{
				RegexValue: n.Value.String(),
			},
		}, nil
	case *semantic.MemberExpression:
		// Sanity check that the object is the objectName identifier
		if ident, ok := n.Object.(*semantic.IdentifierExpression); !ok || ident.Name != objectName {
			return nil, fmt.Errorf("unknown object %q", n.Object)
		}
		switch n.Property {
		case datatypes.FieldKey:
			return &datatypes.Node{
				NodeType: datatypes.NodeTypeTagRef,
				Value: &datatypes.Node_TagRefValue{
					TagRefValue: models.FieldKeyTagKey,
				},
			}, nil
		case datatypes.MeasurementKey:
			return &datatypes.Node{
				NodeType: datatypes.NodeTypeTagRef,
				Value: &datatypes.Node_TagRefValue{
					TagRefValue: models.MeasurementTagKey,
				},
			}, nil
		case datatypes.ValueKey:
			return &datatypes.Node{
				NodeType: datatypes.NodeTypeFieldRef,
				Value: &datatypes.Node_FieldRefValue{
					FieldRefValue: datatypes.ValueKey,
				},
			}, nil

		}
		return &datatypes.Node{
			NodeType: datatypes.NodeTypeTagRef,
			Value: &datatypes.Node_TagRefValue{
				TagRefValue: n.Property,
			},
		}, nil
	case *semantic.DurationLiteral:
		return nil, errors.New("duration literals not supported in storage predicates")
	case *semantic.DateTimeLiteral:
		return nil, errors.New("time literals not supported in storage predicates")
	default:
		return nil, fmt.Errorf("unsupported semantic expression type %T", n)
	}
}

func toComparisonOperator(o ast.OperatorKind) (datatypes.Node_Comparison, error) {
	switch o {
	case ast.EqualOperator:
		return datatypes.ComparisonEqual, nil
	case ast.NotEqualOperator:
		return datatypes.ComparisonNotEqual, nil
	case ast.RegexpMatchOperator:
		return datatypes.ComparisonRegex, nil
	case ast.NotRegexpMatchOperator:
		return datatypes.ComparisonNotRegex, nil
	case ast.StartsWithOperator:
		return datatypes.ComparisonStartsWith, nil
	case ast.LessThanOperator:
		return datatypes.ComparisonLess, nil
	case ast.LessThanEqualOperator:
		return datatypes.ComparisonLessEqual, nil
	case ast.GreaterThanOperator:
		return datatypes.ComparisonGreater, nil
	case ast.GreaterThanEqualOperator:
		return datatypes.ComparisonGreaterEqual, nil
	default:
		return 0, fmt.Errorf("unknown operator %v", o)
	}
}
