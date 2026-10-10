package html

import "strings"

// Foreign-content support per the WHATWG spec sections 13.2.6.1 and
// 13.2.6.2: inside svg/math elements the tree builder keeps element
// and attribute name case, honors self-closing on any element, applies
// no HTML implicit-closing rules, and adjusts known names to their
// camelCase spellings.

// Namespace markers for open-element frames.
const (
	nsHTML = ""
	nsSVG  = "svg"
	nsMath = "math"
)

// foreignRoots are the elements that switch the tree builder into
// foreign content.
var foreignRoots = elementSet("svg", "math")

// svgIntegrationPoints are SVG elements whose children parse with HTML
// rules again (the spec's HTML integration points for SVG).
var svgIntegrationPoints = elementSet("foreignobject", "desc", "title")

// svgTagAdjust maps lowercased SVG tag names to their adjusted
// spellings (spec 13.2.6.1).
var svgTagAdjust = map[string]string{
	"altglyph":            "altGlyph",
	"altglyphdef":         "altGlyphDef",
	"altglyphitem":        "altGlyphItem",
	"animatecolor":        "animateColor",
	"animatemotion":       "animateMotion",
	"animatetransform":    "animateTransform",
	"clippath":            "clipPath",
	"feblend":             "feBlend",
	"fecolormatrix":       "feColorMatrix",
	"fecomponenttransfer": "feComponentTransfer",
	"fecomposite":         "feComposite",
	"feconvolvematrix":    "feConvolveMatrix",
	"fediffuselighting":   "feDiffuseLighting",
	"fedisplacementmap":   "feDisplacementMap",
	"fedistantlight":      "feDistantLight",
	"fedropshadow":        "feDropShadow",
	"feflood":             "feFlood",
	"fefunca":             "feFuncA",
	"fefuncb":             "feFuncB",
	"fefuncg":             "feFuncG",
	"fefuncr":             "feFuncR",
	"fegaussianblur":      "feGaussianBlur",
	"feimage":             "feImage",
	"femerge":             "feMerge",
	"femergenode":         "feMergeNode",
	"femorphology":        "feMorphology",
	"feoffset":            "feOffset",
	"fepointlight":        "fePointLight",
	"fespecularlighting":  "feSpecularLighting",
	"fespotlight":         "feSpotLight",
	"fetile":              "feTile",
	"feturbulence":        "feTurbulence",
	"foreignobject":       "foreignObject",
	"glyphref":            "glyphRef",
	"lineargradient":      "linearGradient",
	"radialgradient":      "radialGradient",
	"textpath":            "textPath",
}

// svgAttrAdjust maps lowercased attribute names to their adjusted SVG
// spellings (spec 13.2.6.2).
var svgAttrAdjust = map[string]string{
	"attributename":       "attributeName",
	"attributetype":       "attributeType",
	"basefrequency":       "baseFrequency",
	"baseprofile":         "baseProfile",
	"calcmode":            "calcMode",
	"clippathunits":       "clipPathUnits",
	"diffuseconstant":     "diffuseConstant",
	"edgemode":            "edgeMode",
	"filterunits":         "filterUnits",
	"glyphref":            "glyphRef",
	"gradienttransform":   "gradientTransform",
	"gradientunits":       "gradientUnits",
	"kernelmatrix":        "kernelMatrix",
	"kernelunitlength":    "kernelUnitLength",
	"keypoints":           "keyPoints",
	"keysplines":          "keySplines",
	"keytimes":            "keyTimes",
	"lengthadjust":        "lengthAdjust",
	"limitingconeangle":   "limitingConeAngle",
	"markerheight":        "markerHeight",
	"markerunits":         "markerUnits",
	"markerwidth":         "markerWidth",
	"maskcontentunits":    "maskContentUnits",
	"maskunits":           "maskUnits",
	"numoctaves":          "numOctaves",
	"pathlength":          "pathLength",
	"patterncontentunits": "patternContentUnits",
	"patterntransform":    "patternTransform",
	"patternunits":        "patternUnits",
	"pointsatx":           "pointsAtX",
	"pointsaty":           "pointsAtY",
	"pointsatz":           "pointsAtZ",
	"preservealpha":       "preserveAlpha",
	"preserveaspectratio": "preserveAspectRatio",
	"primitiveunits":      "primitiveUnits",
	"refx":                "refX",
	"refy":                "refY",
	"repeatcount":         "repeatCount",
	"repeatdur":           "repeatDur",
	"requiredextensions":  "requiredExtensions",
	"requiredfeatures":    "requiredFeatures",
	"specularconstant":    "specularConstant",
	"specularexponent":    "specularExponent",
	"spreadmethod":        "spreadMethod",
	"startoffset":         "startOffset",
	"stddeviation":        "stdDeviation",
	"stitchtiles":         "stitchTiles",
	"surfacescale":        "surfaceScale",
	"systemlanguage":      "systemLanguage",
	"tablevalues":         "tableValues",
	"targetx":             "targetX",
	"targety":             "targetY",
	"textlength":          "textLength",
	"viewbox":             "viewBox",
	"xchannelselector":    "xChannelSelector",
	"ychannelselector":    "yChannelSelector",
	"zoomandpan":          "zoomAndPan",
}

// mathAttrAdjust maps lowercased attribute names to their adjusted
// MathML spellings (spec 13.2.6.1).
var mathAttrAdjust = map[string]string{
	"definitionurl": "definitionURL",
}

// htmlEncodings are the encoding values that make an annotation-xml
// element an HTML integration point.
var htmlEncodings = elementSet("text/html", "application/xhtml+xml")

// childNamespace returns the namespace a new element with the given
// lowercased name and attributes gets when inserted into parentNS.
func childNamespace(parentNS, lower string, attrs []Attribute) string {
	if foreignRoots[lower] {
		if lower == "svg" {
			return nsSVG
		}
		return nsMath
	}
	if parentNS == nsSVG && svgIntegrationPoints[lower] {
		return nsHTML
	}
	if parentNS == nsMath && lower == "annotation-xml" && annotationIsHTML(attrs) {
		return nsHTML
	}
	return parentNS
}

// annotationIsHTML reports whether an annotation-xml element's encoding
// marks it as an HTML integration point. The value is matched
// case-insensitively after trimming whitespace.
func annotationIsHTML(attrs []Attribute) bool {
	for _, attr := range attrs {
		if !strings.EqualFold(attr.Name, "encoding") {
			continue
		}
		return htmlEncodings[strings.ToLower(strings.TrimSpace(attr.Value))]
	}
	return false
}
