from datetime import datetime, timedelta
from airflow import DAG
from airflow.operators.python import PythonOperator, BranchPythonOperator
from airflow.operators.empty import EmptyOperator
import random
import time
import logging

logger = logging.getLogger(__name__)


def validate_payment(**context):
    """Step 1: Validate payment details"""
    order_id = context['dag_run'].conf.get('order_id', 'ORD-001')
    logger.info(f"Validating payments for order {order_id}")
    time.sleep(2) # Simultae api call

    # Simulate 80% chance of success
    if random.random() < 0.2:
        raise Exception(f"Payment validation failed for {order_id}")

    logger.info(f"Payment validated for {order_id}")
    return {"order_id": order_id, "payment_status": "validated"}


def reserve_inventory(**context):
    """Step 2: Reserve items in warehouse."""
    order_id = context['dag_run'].conf.get('order_id', 'ORD-001')
    logger.info(f"Reserving inventory for order {order_id}")
    time.sleep(2)  # Simulate inventory system call

    # Simulate 90% success rate
    if random.random() < 0.1:
        raise Exception(f"Inventory not avilable for {order_id}")

    logger.info(f"Inventory reserved for {order_id}")

    return {"order_id": order_id, "inventory_status": "reserved"}


def process_shipment(**context):
    """Step 3: Create shipment and hand off to carrier."""
    order_id = context['dag_run'].conf.get('order_id', 'ORD-001')
    logger.info(f"Processing shipment for order {order_id}")
    time.sleep(2) # Simulate shipping API

    tracking_id = f"TRK-{random.randint(100000, 9999999)}"
    logger.info(f"Shipment created: {tracking_id} for order {order_id}")
    return {"order_id": order_id, "tracking_id": tracking_id}

def notify_customer(**context):
    """Step 4: Send notification to customer."""
    order_id = context['dag_run'].conf.get('order_id', 'ORD-001')
    logger.info(f"Sending notification to customer for order {order_id}")
    time.sleep(1)  # Simulate email/SMS
    logger.info(f"Customer notified: Order {order_id} is on its way!")
    return {"order_id": order_id, "notification": "sent"}

def check_payment_result(**context):
    """Branch: check if payment succeeded or failed after retries."""
    ti = context['ti']
    try:
        ti.xcom_pull(task_ids='validate_payment')
        return 'reserve_inventory'
    except Exception:
        return 'handle_payment_failure'


def handle_payment_failure(**context):
    """Fallback: handle payment failure (refund, notify)."""
    order_id = context['dag_run'].conf.get('order_id', 'ORD-001')
    logger.info(f"Payment failed for {order_id} after retries. Initiating refund.")
    time.sleep(1)
    logger.info(f"Refund processed for {order_id}. Customer notified.")
    return {"order_id": order_id, "status": "refunded"}


# ============================ DAG Defination ========================================

default_args = {
    'owner': 'jagdeep',
    'retries': 3,
    'retry_delay': timedelta(seconds=20),
    'execution_timeout': timedelta(minutes=5)
}

with DAG(
    dag_id='order_processing_pipeline',
    default_args=default_args,
    description='Order fulfilment: validate -> reserve -> ship -> notify',
    schedule=None, # Manual trigger only
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=['orders', 'demo',]
) as dag:

    # Tasks
    start = EmptyOperator(task_id='start')

    validate = PythonOperator(
        task_id='validate_payment',
        python_callable=validate_payment,
        retries=3,
        retry_delay=timedelta(seconds=5),
    )

    reserve = PythonOperator(
        task_id='reserve_inventory',
        python_callable=reserve_inventory,
        retries=2,
        retry_delay=timedelta(seconds=10),
    )

    ship = PythonOperator(
        task_id='process_shipment',
        python_callable=process_shipment,
    )

    notify = PythonOperator(
        task_id='notify_customer',
        python_callable=notify_customer,
    )

    payment_failed = PythonOperator(
        task_id='handle_payment_failure',
        python_callable=handle_payment_failure,
        trigger_rule='one_failed',
    )


    end_success = EmptyOperator(task_id='end_success')
    end_failure = EmptyOperator(task_id='end_failure')

    # DAG flow
    start >> validate >> reserve >> ship >> notify >> end_success
    validate >> payment_failed >> end_failure
