from setuptools import setup, find_packages
import os

here = os.path.abspath(os.path.dirname(__file__))

with open(os.path.join(here, "README.md"), encoding="utf-8") as f:
    long_description = f.read()

setup(
    name="bap-sdk",
    version="0.1.0",
    description="Zero-dependency Python SDK for Bounded Authority Plane (BAP) AI Agent Governance",
    long_description=long_description,
    long_description_content_type="text/markdown",
    author="Enterprise Security Architecture Team",
    packages=find_packages(include=["bap_sdk", "bap_sdk.*"]),
    python_requires=">=3.8",
    install_requires=[],  # Zero third-party dependencies!
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "Topic :: Security",
        "Programming Language :: Python :: 3",
    ],
)

