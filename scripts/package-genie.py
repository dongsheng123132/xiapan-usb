"""Compatibility entry point for the one portable-master assembly recipe.

Usage: python scripts/package-genie.py --resources <resource-master> --output <new-directory>
"""
from pathlib import Path
import runpy

runpy.run_path(str(Path(__file__).with_name('package-portable.py')), run_name='__main__')
