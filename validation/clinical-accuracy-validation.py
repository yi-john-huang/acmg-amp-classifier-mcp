#!/usr/bin/env python3
"""
Clinical Accuracy Validation Suite for ACMG/AMP MCP Server

This validation suite tests the clinical accuracy of variant classifications
against known reference datasets and expert curated variant interpretations.

Test Categories:
- ClinVar Reference Dataset Validation
- Known Pathogenic Variant Classification
- Known Benign Variant Classification
- Uncertain Significance Variant Handling
- Population Frequency Assessment
- Functional Evidence Integration
- Literature Evidence Evaluation
- ACMG/AMP Criteria Application Accuracy
"""

import asyncio
import csv
import json
import logging
import sys
import time
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Any, Optional, Tuple
import statistics

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

@dataclass
class ClinicalTestCase:
    """Clinical validation test case"""
    variant_id: str
    hgvs: str
    gene: str
    expected_classification: str
    expected_criteria: List[str]
    clinical_context: List[str]
    reference_source: str
    confidence_threshold: float = 0.8

@dataclass 
class ValidationResult:
    """Result of clinical validation"""
    test_case_id: str
    passed: bool
    predicted_classification: str
    expected_classification: str
    confidence_score: float
    applied_criteria: List[str]
    expected_criteria: List[str]
    criteria_accuracy: float
    message: str
    details: Optional[Dict[str, Any]] = None
    duration: Optional[float] = None

@dataclass
class ClinicalReport:
    """Clinical accuracy validation report"""
    timestamp: str
    total_variants: int
    classification_accuracy: float
    criteria_accuracy: float
    high_confidence_accuracy: float
    results_by_classification: Dict[str, Dict[str, int]]
    detailed_results: List[ValidationResult]
    performance_metrics: Dict[str, float]
    recommendations: List[str]

class ClinicalAccuracyValidator:
    """Clinical accuracy validation engine"""
    
    def __init__(self, mcp_client):
        self.mcp_client = mcp_client
        self.results = []
        
    def load_test_cases(self, test_file: str) -> List[ClinicalTestCase]:
        """Load clinical test cases from file"""
        
        test_cases = []
        file_path = Path(test_file)
        
        if file_path.suffix == '.json':
            with open(file_path, 'r') as f:
                data = json.load(f)
                for item in data:
                    test_cases.append(ClinicalTestCase(**item))
        
        elif file_path.suffix == '.csv':
            with open(file_path, 'r') as f:
                reader = csv.DictReader(f)
                for row in reader:
                    test_cases.append(ClinicalTestCase(
                        variant_id=row['variant_id'],
                        hgvs=row['hgvs'],
                        gene=row['gene'],
                        expected_classification=row['expected_classification'],
                        expected_criteria=row['expected_criteria'].split(',') if row['expected_criteria'] else [],
                        clinical_context=row['clinical_context'].split(',') if row['clinical_context'] else [],
                        reference_source=row['reference_source'],
                        confidence_threshold=float(row.get('confidence_threshold', 0.8))
                    ))
        else:
            raise ValueError(f"Unsupported test file format: {file_path.suffix}")
            
        logger.info(f"Loaded {len(test_cases)} test cases from {test_file}")
        return test_cases
    
    def create_default_test_cases(self) -> List[ClinicalTestCase]:
        """Create default test cases for common variants"""
        
        return [
            # Well-known pathogenic variants
            ClinicalTestCase(
                variant_id="CFTR_F508del",
                hgvs="NM_000492.3:c.1521_1523delCTT",
                gene="CFTR", 
                expected_classification="Pathogenic",
                expected_criteria=["PVS1", "PS3", "PM2", "PP3", "PP5"],
                clinical_context=["cystic_fibrosis"],
                reference_source="ClinVar_4star",
                confidence_threshold=0.95
            ),
            ClinicalTestCase(
                variant_id="BRCA1_185delA", 
                hgvs="NM_007294.3:c.185delA",
                gene="BRCA1",
                expected_classification="Pathogenic",
                expected_criteria=["PVS1", "PM2", "PP5"],
                clinical_context=["hereditary_breast_ovarian_cancer"],
                reference_source="ClinVar_4star", 
                confidence_threshold=0.95
            ),
            ClinicalTestCase(
                variant_id="BRCA2_6174delT",
                hgvs="NM_000059.3:c.6174delT", 
                gene="BRCA2",
                expected_classification="Pathogenic",
                expected_criteria=["PVS1", "PM2", "PP5"],
                clinical_context=["hereditary_breast_ovarian_cancer"],
                reference_source="ClinVar_4star",
                confidence_threshold=0.95
            ),
            
            # Well-known benign variants
            ClinicalTestCase(
                variant_id="CFTR_benign_polymorphism",
                hgvs="NM_000492.3:c.1540A>G", 
                gene="CFTR",
                expected_classification="Benign",
                expected_criteria=["BA1", "BS1"],
                clinical_context=[],
                reference_source="ClinVar_expert_panel",
                confidence_threshold=0.90
            ),
            
            # Likely pathogenic variants
            ClinicalTestCase(
                variant_id="FBN1_missense",
                hgvs="NM_000138.4:c.419G>A",
                gene="FBN1", 
                expected_classification="Likely Pathogenic",
                expected_criteria=["PM1", "PM2", "PP2", "PP3"],
                clinical_context=["marfan_syndrome"],
                reference_source="ClinVar_multiple_submitters",
                confidence_threshold=0.85
            ),
            
            # Uncertain significance variants
            ClinicalTestCase(
                variant_id="BRCA1_VUS",
                hgvs="NM_007294.3:c.4096G>A",
                gene="BRCA1",
                expected_classification="Uncertain Significance", 
                expected_criteria=[],
                clinical_context=["breast_cancer_risk"],
                reference_source="ClinVar_conflicting",
                confidence_threshold=0.60
            ),
            
            # Likely benign variants
            ClinicalTestCase(
                variant_id="BRCA2_likely_benign",
                hgvs="NM_000059.3:c.7436T>C",
                gene="BRCA2",
                expected_classification="Likely Benign", 
                expected_criteria=["BS1", "BP1"],
                clinical_context=[],
                reference_source="ClinVar_expert_panel",
                confidence_threshold=0.85
            ),
        ]
    
    async def classify_variant(self, test_case: ClinicalTestCase) -> Dict[str, Any]:
        """Classify variant using MCP server"""
        
        result = await self.mcp_client.call_tool("classify_variant", {
            "variant_data": {
                "hgvs": test_case.hgvs,
                "gene": test_case.gene
            },
            "options": {
                "include_evidence": True,
                "confidence_threshold": test_case.confidence_threshold,
                "clinical_context": test_case.clinical_context
            }
        })
        
        return result
    
    def calculate_criteria_accuracy(self, predicted: List[str], expected: List[str]) -> float:
        """Calculate ACMG/AMP criteria prediction accuracy"""
        
        if not expected:
            return 1.0  # No specific criteria expected
            
        predicted_set = set(predicted)
        expected_set = set(expected)
        
        if not predicted_set and not expected_set:
            return 1.0
            
        # Calculate overlap
        intersection = predicted_set & expected_set
        union = predicted_set | expected_set
        
        # Jaccard similarity
        accuracy = len(intersection) / len(union) if union else 0.0
        
        return accuracy
    
    def classify_prediction_accuracy(self, predicted: str, expected: str) -> bool:
        """Determine if classification prediction is accurate"""
        
        # Exact match
        if predicted == expected:
            return True
            
        # Allow some flexibility for borderline cases
        acceptable_mappings = {
            "Pathogenic": ["Likely Pathogenic"],
            "Likely Pathogenic": ["Pathogenic", "Uncertain Significance"], 
            "Uncertain Significance": ["Likely Pathogenic", "Likely Benign"],
            "Likely Benign": ["Benign", "Uncertain Significance"],
            "Benign": ["Likely Benign"]
        }
        
        return predicted in acceptable_mappings.get(expected, [])
    
    async def validate_test_case(self, test_case: ClinicalTestCase) -> ValidationResult:
        """Validate single test case"""
        
        start_time = time.time()
        
        try:
            # Classify variant
            classification_result = await self.classify_variant(test_case)
            
            predicted_classification = classification_result.get("classification", "Unknown")
            confidence_score = classification_result.get("confidence", 0.0)
            applied_criteria = classification_result.get("applied_criteria", [])
            
            # Calculate accuracy metrics
            classification_accurate = self.classify_prediction_accuracy(
                predicted_classification, 
                test_case.expected_classification
            )
            
            criteria_accuracy = self.calculate_criteria_accuracy(
                applied_criteria,
                test_case.expected_criteria
            )
            
            duration = time.time() - start_time
            
            # Determine overall pass/fail
            passed = (
                classification_accurate and 
                confidence_score >= test_case.confidence_threshold - 0.1  # Allow small tolerance
            )
            
            message = "Classification accurate" if passed else "Classification mismatch"
            if not classification_accurate:
                message = f"Expected {test_case.expected_classification}, got {predicted_classification}"
            elif confidence_score < test_case.confidence_threshold:
                message = f"Low confidence: {confidence_score:.2f} < {test_case.confidence_threshold}"
                
            return ValidationResult(
                test_case_id=test_case.variant_id,
                passed=passed,
                predicted_classification=predicted_classification,
                expected_classification=test_case.expected_classification,
                confidence_score=confidence_score,
                applied_criteria=applied_criteria,
                expected_criteria=test_case.expected_criteria,
                criteria_accuracy=criteria_accuracy,
                message=message,
                details={
                    "hgvs": test_case.hgvs,
                    "gene": test_case.gene,
                    "reference_source": test_case.reference_source,
                    "clinical_context": test_case.clinical_context,
                    "full_result": classification_result
                },
                duration=duration
            )
            
        except Exception as e:
            duration = time.time() - start_time
            
            return ValidationResult(
                test_case_id=test_case.variant_id,
                passed=False,
                predicted_classification="Error",
                expected_classification=test_case.expected_classification,
                confidence_score=0.0,
                applied_criteria=[],
                expected_criteria=test_case.expected_criteria,
                criteria_accuracy=0.0,
                message=f"Classification failed: {str(e)}",
                details={
                    "error": str(e),
                    "hgvs": test_case.hgvs,
                    "gene": test_case.gene
                },
                duration=duration
            )
    
    async def run_validation(self, test_cases: List[ClinicalTestCase]) -> ClinicalReport:
        """Run clinical accuracy validation"""
        
        logger.info(f"Starting clinical accuracy validation with {len(test_cases)} test cases")
        
        results = []
        
        for i, test_case in enumerate(test_cases, 1):
            logger.info(f"Validating {i}/{len(test_cases)}: {test_case.variant_id}")
            
            result = await self.validate_test_case(test_case)
            results.append(result)
            
            # Log result
            status = "✅ PASS" if result.passed else "❌ FAIL" 
            logger.info(f"{status}: {result.test_case_id} - {result.message}")
            
            # Brief pause to avoid overwhelming server
            await asyncio.sleep(0.1)
        
        # Calculate overall metrics
        total_variants = len(results)
        classification_accuracy = sum(1 for r in results if r.passed) / total_variants if total_variants > 0 else 0
        
        criteria_accuracy = sum(r.criteria_accuracy for r in results) / total_variants if total_variants > 0 else 0
        
        high_confidence_results = [r for r in results if r.confidence_score >= 0.9]
        high_confidence_accuracy = (
            sum(1 for r in high_confidence_results if r.passed) / len(high_confidence_results)
            if high_confidence_results else 0
        )
        
        # Results by classification type
        results_by_classification = {}
        classifications = ["Pathogenic", "Likely Pathogenic", "Uncertain Significance", "Likely Benign", "Benign"]
        
        for cls in classifications:
            cls_results = [r for r in results if r.expected_classification == cls]
            if cls_results:
                correct = sum(1 for r in cls_results if r.passed)
                results_by_classification[cls] = {
                    "total": len(cls_results),
                    "correct": correct,
                    "accuracy": correct / len(cls_results)
                }
        
        # Performance metrics
        performance_metrics = {
            "average_response_time": statistics.mean(r.duration for r in results if r.duration),
            "median_response_time": statistics.median(r.duration for r in results if r.duration), 
            "max_response_time": max(r.duration for r in results if r.duration),
            "average_confidence": statistics.mean(r.confidence_score for r in results),
            "median_confidence": statistics.median(r.confidence_score for r in results)
        }
        
        # Generate recommendations
        recommendations = []
        
        if classification_accuracy < 0.95:
            recommendations.append("Classification accuracy below 95% - review failed cases for systematic issues")
            
        if criteria_accuracy < 0.80:
            recommendations.append("ACMG/AMP criteria accuracy below 80% - review criteria application logic")
            
        if performance_metrics["average_response_time"] > 5.0:
            recommendations.append("Average response time > 5 seconds - consider performance optimization")
            
        # Check for specific classification issues
        for cls, metrics in results_by_classification.items():
            if metrics["accuracy"] < 0.90:
                recommendations.append(f"{cls} classification accuracy low ({metrics['accuracy']:.1%}) - review {cls.lower()} detection")
        
        if not recommendations:
            recommendations.append("All clinical accuracy metrics meet or exceed targets")
        
        return ClinicalReport(
            timestamp=datetime.now(timezone.utc).isoformat(),
            total_variants=total_variants,
            classification_accuracy=classification_accuracy,
            criteria_accuracy=criteria_accuracy,
            high_confidence_accuracy=high_confidence_accuracy,
            results_by_classification=results_by_classification,
            detailed_results=results,
            performance_metrics=performance_metrics,
            recommendations=recommendations
        )

def format_clinical_report(report: ClinicalReport) -> str:
    """Format clinical accuracy report"""
    
    accuracy_emoji = "✅" if report.classification_accuracy >= 0.95 else "⚠️" if report.classification_accuracy >= 0.90 else "❌"
    
    output = f"""
{accuracy_emoji} CLINICAL ACCURACY VALIDATION REPORT {accuracy_emoji}

📅 Validation Date: {report.timestamp}
🧬 Total Variants Tested: {report.total_variants}
🎯 Overall Classification Accuracy: {report.classification_accuracy:.1%}
📋 ACMG/AMP Criteria Accuracy: {report.criteria_accuracy:.1%}
⭐ High Confidence Accuracy: {report.high_confidence_accuracy:.1%}

{'='*80}
CLASSIFICATION BREAKDOWN
{'='*80}
"""
    
    for classification, metrics in report.results_by_classification.items():
        accuracy_str = f"{metrics['accuracy']:.1%}"
        output += f"{classification:>20}: {metrics['correct']:>3}/{metrics['total']:>3} ({accuracy_str})\n"
    
    output += f"\n{'='*80}\nDETAILED RESULTS\n{'='*80}\n"
    
    for result in report.detailed_results:
        status = "✅ PASS" if result.passed else "❌ FAIL"
        output += f"\n{status} {result.test_case_id}\n"
        output += f"    Variant: {result.details['hgvs']} ({result.details['gene']})\n"
        output += f"    Expected: {result.expected_classification}\n"
        output += f"    Predicted: {result.predicted_classification} (confidence: {result.confidence_score:.2f})\n"
        output += f"    Criteria Accuracy: {result.criteria_accuracy:.2f}\n"
        output += f"    Applied Criteria: {', '.join(result.applied_criteria) if result.applied_criteria else 'None'}\n"
        output += f"    Expected Criteria: {', '.join(result.expected_criteria) if result.expected_criteria else 'None'}\n"
        output += f"    Response Time: {result.duration:.3f}s\n"
        output += f"    Message: {result.message}\n"
        
        if not result.passed and result.details.get('clinical_context'):
            output += f"    Clinical Context: {', '.join(result.details['clinical_context'])}\n"
    
    output += f"\n{'='*80}\nPERFORMANCE METRICS\n{'='*80}\n"
    
    perf = report.performance_metrics
    output += f"Average Response Time: {perf['average_response_time']:.3f}s\n"
    output += f"Median Response Time: {perf['median_response_time']:.3f}s\n"
    output += f"Maximum Response Time: {perf['max_response_time']:.3f}s\n"
    output += f"Average Confidence: {perf['average_confidence']:.3f}\n"
    output += f"Median Confidence: {perf['median_confidence']:.3f}\n"
    
    output += f"\n{'='*80}\nRECOMMENDATIONS\n{'='*80}\n"
    
    for i, recommendation in enumerate(report.recommendations, 1):
        output += f"{i}. {recommendation}\n"
    
    return output

# Mock MCP Client for testing
class MockMCPClient:
    """Mock MCP client for testing validation framework"""
    
    async def call_tool(self, tool_name: str, params: Dict[str, Any]) -> Dict[str, Any]:
        """Mock tool call implementation"""
        
        if tool_name == "classify_variant":
            variant_data = params.get("variant_data", {})
            hgvs = variant_data.get("hgvs", "")
            
            # Mock responses for known variants
            if "c.1521_1523delCTT" in hgvs:  # F508del
                return {
                    "classification": "Pathogenic",
                    "confidence": 0.98,
                    "applied_criteria": ["PVS1", "PS3", "PM2", "PP3", "PP5"],
                    "evidence_summary": {
                        "PVS1": "Null variant in gene where LOF is disease mechanism",
                        "PS3": "Well-established functional studies show damaging effect",
                        "PM2": "Absent from controls in population databases",
                        "PP3": "Computational evidence supports deleterious effect", 
                        "PP5": "Reputable source reports variant as pathogenic"
                    }
                }
            elif "c.185delA" in hgvs:  # BRCA1
                return {
                    "classification": "Pathogenic", 
                    "confidence": 0.96,
                    "applied_criteria": ["PVS1", "PM2", "PP5"],
                    "evidence_summary": {}
                }
            else:
                # Generic uncertain response
                return {
                    "classification": "Uncertain Significance",
                    "confidence": 0.65,
                    "applied_criteria": [],
                    "evidence_summary": {}
                }
        
        return {}

async def main():
    """Main validation function"""
    
    # For testing purposes, use mock client
    # In production, this would connect to actual MCP server
    mcp_client = MockMCPClient()
    
    validator = ClinicalAccuracyValidator(mcp_client)
    
    # Load test cases
    if len(sys.argv) > 1:
        test_cases = validator.load_test_cases(sys.argv[1])
    else:
        logger.info("No test file provided, using default test cases")
        test_cases = validator.create_default_test_cases()
    
    # Run validation
    report = await validator.run_validation(test_cases)
    
    # Display results
    print(format_clinical_report(report))
    
    # Save detailed report
    report_file = f"clinical_accuracy_report_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(report_file, 'w') as f:
        report_dict = asdict(report)
        json.dump(report_dict, f, indent=2, default=str)
    
    print(f"\nDetailed report saved to: {report_file}")
    
    # Exit with appropriate code
    success_threshold = 0.95
    sys.exit(0 if report.classification_accuracy >= success_threshold else 1)

if __name__ == "__main__":
    asyncio.run(main())