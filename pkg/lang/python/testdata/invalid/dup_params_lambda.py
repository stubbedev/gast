# lambdas share the duplicate-parameter rule, across *args too
g = lambda x, *x: x
