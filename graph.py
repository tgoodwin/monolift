import os
import pandas as pd

import matplotlib.pyplot as plt

def plot_experiment_results(results_folder):
    # Initialize the plot
    plt.figure(figsize=(10, 6))
    
    # Traverse through the results folder and its subdirectories
    for root, _, files in os.walk(results_folder):
        for file in files:
            if file.endswith(".csv"):
                file_path = os.path.join(root, file)
                
                # Read the CSV file
                try:
                    data = pd.read_csv(file_path)
                    
                    # Ensure the required columns are present
                    if "Actual RPS" in data.columns and "p95 Latency (ms)" in data.columns:
                        # Plot the data
                        plt.plot(data["Actual RPS"], data["p95 Latency (ms)"], label=os.path.relpath(file_path, results_folder), linestyle='-', marker='o')
                    else:
                        print(f"Skipping {file_path}: Missing required columns.")
                except Exception as e:
                    print(f"Error reading {file_path}: {e}")


    # Customize the plot
    plt.title("Experiment Results")
    plt.ylim(0, 20)  # Adjust the y-axis limit as needed
    # plt.yscale("log")
    plt.xscale("log")
    plt.xlabel("Actual RPS")
    plt.ylabel("p95 Latency")
    plt.legend(loc="best", fontsize="small")
    plt.grid(True)
    
    # Show the plot
    plt.tight_layout()
    plt.savefig(os.path.join(results_folder, "experiment_results.png"))
    plt.show()

if __name__ == "__main__":
    results_folder = "./results"  # Adjust the path if needed
    plot_experiment_results(results_folder)