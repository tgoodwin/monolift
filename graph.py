import os
import pandas as pd
import argparse

import matplotlib.pyplot as plt

def plot_experiment_results(results_folder, x_column, y_column, aggregate,run_set):
  # Initialize the plot
    plt.figure(figsize=(4 ,4))
        
    dfs = list() # List to hold DataFrames for each CSV file

    # Traverse through the results folder and its subdirectories and ingest data
    for root, _, files in os.walk(results_folder):
        for file in files:
            if file.endswith(".csv"):
                file_path = os.path.join(root, file) 
                full_name = os.path.relpath(file_path, results_folder)
                series_name = os.path.dirname(full_name)
                if series_name not in run_set:
                    continue
                
                # Read the CSV file
                try:
                    data = pd.read_csv(file_path)
                    
                    # Ensure the required columns are present
                    if x_column in data.columns and y_column in data.columns:
                        # Plot the data

                        data['series_name'] = series_name  # Add series name to the DataFrame
                        data['run'] = file.split('-')[1].split('.')[0]  # Add run name from the file name, assuming name is throughput-1.csv
                        dfs.append(data)  # Append the DataFrame to the list
                    else:
                        print(f"Skipping {file_path}: Missing required columns.")
                except Exception as e:
                    print(f"Error reading {file_path}: {e}")

    if not dfs:
        print("No data files found. Exiting.")
        return

    

    # Aggregate dataframes by column values for each series name
    all_data = pd.concat(dfs, ignore_index=True)

    if aggregate:
        # Group by series and x_column (e.g., "Actual RPS")
        grouped = all_data.groupby(['series_name', x_column])

        # Aggregate mean and std dev
        aggregated = grouped[y_column].agg(['mean', 'std']).reset_index()

        # Plot each series separately
        for series_name, group_df in aggregated.groupby('series_name'):
            x_vals = group_df[x_column]
            y_mean = group_df['mean']
            y_std = group_df['std']

            # Plot with error bars
            # plt.errorbar(x_vals, y_mean, yerr=y_std, label=series_name, capsize=3, marker='x', linestyle='-',linewidth=1)
            # Plot the mean line for the series
            plt.plot(x_vals, y_mean, label=series_name, marker='x', linestyle='-', linewidth=1)

            # Fill the area between mean - std and mean + std
            plt.fill_between(x_vals, y_mean - y_std, y_mean + y_std, alpha=0.2)
        
        # Customize the plot
        plt.ylim(0, 15)  # Adjust the y-axis limit as needed
        # plt.yscale("log")
        plt.xscale("log")
        plt.xlabel(x_column)
        plt.ylabel(y_column)
        plt.legend(loc="best", fontsize=8)
        plt.grid(True)
        
        file_name = x_column + "_vs_" + y_column + ".png"
        
        # Show the plot
        plt.tight_layout(pad=0.1, rect=[0, 0, 1, 1]) 
        output_file = os.path.join(results_folder, file_name)
        plt.savefig(output_file.replace('.png', '.pdf'), dpi=600)
    else:
        runs = all_data['run'].unique()
        for run in sorted(runs):
            run_df = all_data[all_data['run'] == run]

            plt.figure(figsize=(4, 4))

            for series_name, group_df in run_df.groupby('series_name'):
                if x_column in group_df.columns and y_column in group_df.columns:
                    x_vals = group_df[x_column]
                    y_vals = group_df[y_column]

                    label = f"{series_name}"
                    plt.plot(x_vals, y_vals, label=label, marker='x', linestyle='-', linewidth=1)

            # Plot settings
            plt.xscale("log")
            plt.xlabel(x_column)
            plt.ylabel(y_column)
            plt.ylim(0, 100)
            plt.grid(True)
            plt.legend(loc="best", fontsize=8)
            plt.tight_layout(pad=0.1, rect=[0, 0, 1, 1])

            # Output file
            file_base = f"run_{run}_{x_column}_vs_{y_column}".replace(" ", "_")
            output_file = os.path.join(results_folder, file_base + ".pdf")
            plt.savefig(output_file, dpi=600)
            plt.close()


if __name__ == "__main__":

    # Set up argument parser
    parser = argparse.ArgumentParser(description="Plot experiment results from CSV files.")
    parser.add_argument("results_folder",nargs='?', const=1,default="results", type=str, help="Path to the folder containing the results.")
    parser.add_argument("x_column", nargs='?',const=1,default="Actual RPS", type=str, help="Column name to use for the x-axis.")
    parser.add_argument("y_column", nargs='?',const=1,default="Avg Latency (ms)", type=str, help="Column name to use for the y-axis.")
    parser.add_argument("--aggregate", dest="aggregate", action="store_true", help="Aggregate multiple runs by averaging.")
    parser.add_argument("--no-aggregate", dest="aggregate", action="store_false", help="Plot individual runs without aggregation.")
    parser.set_defaults(aggregate=True)


    run_set = ["full","monolith", "monolith_large", "post", "user", "timeline", "socialgraph", "mixed_profile_half_peak", "save_profile_peak", "save_profile_half_peak"]
    # run_set = ["full","monolith", "post", "user", "timeline", "socialgraph"]

    
    # Parse arguments
    args = parser.parse_args()
    print(f"Results folder: {args.results_folder}")
    print(f"x-axis column: {args.x_column}")
    print(f"y-axis column: {args.y_column}")
    # Call the function with the provided arguments
    plot_experiment_results(args.results_folder, args.x_column, args.y_column, args.aggregate,run_set)