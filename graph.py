import os
import pandas as pd
import argparse

import matplotlib.pyplot as plt

def plot_experiment_results(results_folder, x_column, y_column):
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
                    if x_column in data.columns and y_column in data.columns:
                        # Plot the data
                        plt.plot(data[x_column], data[y_column], label=os.path.relpath(file_path, results_folder), linestyle='-', marker='o')
                    else:
                        print(f"Skipping {file_path}: Missing required columns.")
                except Exception as e:
                    print(f"Error reading {file_path}: {e}")


    # Customize the plot
    plt.title("Experiment Results")
    plt.ylim(0, 200)  # Adjust the y-axis limit as needed
    # plt.yscale("log")
    plt.xscale("log")
    plt.xlabel(x_column)
    plt.ylabel(y_column)
    plt.legend(loc="best", fontsize="small")
    plt.grid(True)
    
    file_name = x_column + "_vs_" + y_column + ".png"

    # Show the plot
    plt.tight_layout()
    plt.savefig(os.path.join(results_folder, file_name))
    plt.show()

if __name__ == "__main__":

    # Set up argument parser
    parser = argparse.ArgumentParser(description="Plot experiment results from CSV files.")
    parser.add_argument("results_folder",nargs='?', const=1,default="results", type=str, help="Path to the folder containing the results.")
    parser.add_argument("x_column", nargs='?',const=1,default="Actual RPS", type=str, help="Column name to use for the x-axis.")
    parser.add_argument("y_column", nargs='?',const=1,default="Avg Latency (ms)", type=str, help="Column name to use for the y-axis.")
    
    # Parse arguments
    args = parser.parse_args()
    print(f"Results folder: {args.results_folder}")
    print(f"x-axis column: {args.x_column}")
    print(f"y-axis column: {args.y_column}")
    # Call the function with the provided arguments
    plot_experiment_results(args.results_folder, args.x_column, args.y_column)