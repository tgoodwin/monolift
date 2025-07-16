import asyncio
import aiohttp
import time
import math
import random
import string
import os
import argparse
import csv
import numpy as np

# Try to import matplotlib, but don't make it a hard requirement
try:
    import matplotlib.pyplot as plt
    MATPLOTLIB_AVAILABLE = True
except ImportError:
    MATPLOTLIB_AVAILABLE = False

def sigmoid(x, start_val, end_val, total_duration):
    """
    Calculates a point on a sigmoid curve.
    x: current time step (from 0 to total_duration)
    start_val: the value at x=0
    end_val: the value at x=total_duration
    total_duration: the total length of the curve
    """
    if x < 0 or x > total_duration:
        raise ValueError("x must be within the range [0, total_duration]")
    
    k = 10
    x_shifted = x - (total_duration / 2)
    logistic_val = 1 / (1 + math.exp(-k * x_shifted / total_duration))
    return start_val + (end_val - start_val) * logistic_val


def generate_compose_post_data(num_total_users):
    """
    Creates an aiohttp.FormData object for a compose post request.
    """
    data = aiohttp.FormData()
    user_id = str(random.randint(0, num_total_users - 1))
    text = ''.join(random.choices(string.ascii_letters + string.digits, k=256))
    data.add_field('user_id', user_id)
    data.add_field('text', text)
    num_images = random.randint(1, 4)
    dummy_image_data = os.urandom(1024)
    for i in range(num_images):
        data.add_field(
            'images',
            dummy_image_data,
            filename=f'image_{i}.jpg',
            content_type='image/jpeg'
        )
    return data


async def send_request(session, url, num_total_users, results_queue):
    """Sends a single request and puts its status and latency in the results queue."""
    start_time = time.time()
    try:
        data = generate_compose_post_data(num_total_users)
        async with session.post(url, data=data, timeout=aiohttp.ClientTimeout(total=30)) as response:
            latency = (time.time() - start_time) * 1000  # ms
            await results_queue.put((response.status, latency))
    except Exception as e:
        await results_queue.put((e, 0))


def plot_results(time_data, latency_data, rps_data, output_filename):
    """Generates and saves a dual-axis plot of latency and offered RPS over time."""
    fig, ax1 = plt.subplots(figsize=(12, 6))

    # Plot Latency on the left Y-axis
    color = 'tab:red'
    ax1.set_xlabel('Time (s)')
    ax1.set_ylabel('Avg Latency (ms)', color=color)
    ax1.plot(time_data, latency_data, color=color, label='Avg Latency')
    ax1.tick_params(axis='y', labelcolor=color)
    ax1.grid(True)

    # Create a second Y-axis for the Offered RPS
    ax2 = ax1.twinx()
    color = 'tab:blue'
    ax2.set_ylabel('Offered RPS', color=color)
    ax2.plot(time_data, rps_data, color=color, linestyle='--', label='Offered RPS')
    ax2.tick_params(axis='y', labelcolor=color)

    fig.suptitle('System Latency vs. Offered Load Over Time', fontsize=16)
    fig.tight_layout(rect=[0, 0, 1, 0.96])  # Adjust layout to make room for title
    
    # Add a single legend for both lines
    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax2.legend(lines + lines2, labels + labels2, loc='upper left')

    plt.savefig(output_filename)
    print(f"\nPlot saved to {output_filename}")


async def main(args):
    """The main function to orchestrate the scaling test."""
    url = f"http://{args.ip}:{args.port}/save"

    print("--- Scaling Test Runner ---")
    print(f"Target URL: {url}")
    print(f"Test Duration: {args.duration}s")
    print(f"Load Shape: Sigmoid from {args.start_rps} to {args.end_rps} RPS")
    if args.output_file:
        print(f"Output CSV file: {args.output_file}")
    if args.plot:
        if not MATPLOTLIB_AVAILABLE:
            print("Warning: --plot was specified, but matplotlib is not installed. Skipping plot generation.")
            print("Please run 'pip install matplotlib' to enable plotting.")
        else:
            print("Plot generation enabled.")
    print("-" * 80)

    csv_file = None
    csv_writer = None
    if args.output_file:
        csv_file = open(args.output_file, 'w', newline='')
        csv_writer = csv.writer(csv_file)
        header = [
            "Time (s)", "Offered RPS", "Actual RPS", "Avg Latency (ms)", 
            "p95 Latency (ms)", "Success Count", "Failure Count"
        ]
        csv_writer.writerow(header)

    print(
        "Time (s) | Offered RPS | Actual RPS | Avg Latency (ms) | p95 Latency (ms) | Success | Fail"
    )
    print(
        "---------+-------------+------------+------------------+------------------+---------+------"
    )
    
    # Data storage for plotting
    plot_time = []
    plot_avg_latency = []
    plot_offered_rps = []

    async with aiohttp.ClientSession() as session:
        print("Warming up the service (5 requests over 5 seconds)...")
        warmup_queue = asyncio.Queue()
        for _ in range(5):
            await send_request(session, url, args.num_users, warmup_queue)
            await asyncio.sleep(1)
        print("Warm-up complete. Starting main test.")
        print("-" * 80)
        
        for second in range(args.duration):
            loop_start_time = time.time()
            
            offered_rps = sigmoid(second, args.start_rps, args.end_rps, args.duration)
            num_requests_to_send = int(offered_rps)

            results_queue = asyncio.Queue()
            
            tasks = []
            if num_requests_to_send > 0:
                delay = 1.0 / num_requests_to_send
                for _ in range(num_requests_to_send):
                    task = asyncio.create_task(send_request(session, url, args.num_users, results_queue))
                    tasks.append(task)
                    await asyncio.sleep(delay)
            else:
                await asyncio.sleep(1.0)

            if tasks:
                await asyncio.gather(*tasks)

            results = []
            while not results_queue.empty():
                results.append(results_queue.get_nowait())

            latencies = []
            success_count = 0
            failure_count = 0
            
            for result, latency in results:
                if isinstance(result, int) and 200 <= result < 300:
                    success_count += 1
                    latencies.append(latency)
                else:
                    failure_count += 1

            actual_rps = success_count
            avg_latency = np.mean(latencies) if latencies else 0
            p95_latency = np.percentile(latencies, 95) if latencies else 0

            print(
                f"{second:8d} | "
                f"{offered_rps:11.1f} | "
                f"{actual_rps:10.1f} | "
                f"{avg_latency:16.1f} | "
                f"{p95_latency:16.1f} | "
                f"{success_count:7d} | "
                f"{failure_count:4d}"
            )

            if csv_writer:
                row = [
                    second, f"{offered_rps:.1f}", f"{actual_rps:.1f}", f"{avg_latency:.1f}",
                    f"{p95_latency:.1f}", success_count, failure_count
                ]
                csv_writer.writerow(row)
            
            if args.plot and MATPLOTLIB_AVAILABLE:
                plot_time.append(second)
                plot_avg_latency.append(avg_latency)
                plot_offered_rps.append(offered_rps)
            
            loop_duration = time.time() - loop_start_time
            if loop_duration < 1.0:
                await asyncio.sleep(1.0 - loop_duration)

    if csv_file:
        csv_file.close()
        print(f"\nResults saved to {args.output_file}")

    if args.plot and MATPLOTLIB_AVAILABLE:
        plot_filename = "scaling_plot.png"
        if args.output_file:
            base, _ = os.path.splitext(args.output_file)
            plot_filename = base + ".png"
        plot_results(plot_time, plot_avg_latency, plot_offered_rps, plot_filename)


if __name__ == "__main__":
    parser = argparse.ArgumentParser('Scaling Test Runner')
    parser.add_argument("--num-users", type=int, default=962, help="Total number of users in the social graph")
    parser.add_argument("--duration", type=int, default=60, help="Total duration of the test in seconds")
    parser.add_argument("--start-rps", type=float, default=10, help="Starting RPS for the sigmoid load shape")
    parser.add_argument("--end-rps", type=float, default=1000, help="Ending RPS for the sigmoid load shape")
    parser.add_argument("--output-file", type=str, default=None, help="Optional path to the output CSV file")
    parser.add_argument('--ip', help='IP address of the target server.', default='127.0.0.1')
    parser.add_argument('--port', help='IP port of the target server.', default=8080)
    parser.add_argument('--plot', action='store_true', help='If specified, generates a plot of the results.')
    args = parser.parse_args()

    random.seed(1)
    asyncio.run(main(args))